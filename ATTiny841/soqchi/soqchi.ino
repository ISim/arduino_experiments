#include <avr/io.h>
#include <avr/sleep.h> //Needed for sleep_mode
#include <avr/wdt.h> //Needed to enable/disable watch dog timer
#include <avr/interrupt.h>  //needed for watchdog interrupt

#define MODEM_ENABLED    true             // nastavit na false pro ladění, aby se moc neposílalo zpráv
#define EXTERNAL_ALARM   true             // pokud je true, tak se při alarmu odesílá žádost o downlink
//  - pokud downlink vrátí první bajt 01 - je vstup povolen a ext. siréna se nevyvolá

#define CLOSE_DOOR_DELAY_MILLIS 120000    // doba po uzavření, kdy se nuluje alarm a odesílá se info o zavření
#define CLEAR_EXT_ALARM_TIMEOUT 60000     // pokud je aktivován ext. alarm a není drženo "test" tlačítko

#define SENSOR_PIN        PIN_PB1       // pin pro interrupt  (senzor)     
#define INTR_LED          PIN_PB2       // dioda signalizující že nastalo přerušení od senzoru
#define SIGNAL_LED_PIN    PIN_PA0       // signalizační dioda - informace
#define MODEM_WAKEUP_PIN  PIN_PB0       // reset modemu  
#define BUTTON_PIN        PIN_PA7       // info tlačítko   
#define SIGNAL_EXT_ALARM  PIN_PA5       // signál pro spuštění sirény (externí alarm)


#define AT_SLEEP        "AT$P=2\n"
#define AT_CHECK        "AT\n"
#define AT_OUT_OF_BAND  "AT$SO\n"
#define AT_SEND_FRAME   "AT$SF="
#define AT_TEMP         "AT$T?\n"
#define AT_VOLTAGE      "AT$V?\n"

#define LED_OK()      infoLED(0b10101011100110)        // OK
#define LED_READY()   infoLED(0b011001)                // R    - Ready
#define LED_ALARM()   infoLED(0b01101101100101111010)  // ALRM - alarm (pouze, pokud je zapnutý info mod)         
#define LED_ERROR()   infoLED(0b011101100111011001)    // ERR  - chyba při inicializaci
#define LED_TEST()    infoLED(0b10110101011110)        // TST  - signalizace testu - bude odeslána info zpráva
#define LED_INFO()    infoLED(0b0101)                  // I    - potvrzení režimu "Info"

#define STATUS_ALARM        0x80
#define STATUS_DOOR_CLOSED  0x00
#define STATUS_DOOR_OPEN    0x01
#define STATUS_INFO         0x40
#define STATUS_HEARTBEAT    0x20

uint32_t volatile watchdogTimer = 10;               // tikáček - 10 je nastaveno, aby se neblokovalo odeslání info zprávy  po startu
bool volatile     doorOpenEvent = false;            // do true přepne otevření dveří

bool              alarm         = false;            // stav "poplachu"
bool              accessAllowed = false;            // indikuje informaci o povoleném vstupu z downlinku
unsigned long     closeTimeOK;                      // čas pro smazání alarmu - nastaví zavření dveří jako millis()+CLOSE_DOOR_DELAY_MILLIS
unsigned long     alarmTime;                        // čas vyvolání alarmu
bool              infoMode;                         // pokud se při startu po v době rozsvícení diody stiskne tlačítko "check",
// zapiná se "info" režim

// ----------------------------------------------
bool isDoorClosed() {
  return digitalRead(SENSOR_PIN) == HIGH;
}

byte encodeDoorState(byte state) {
  return state | ( isDoorClosed() ? STATUS_DOOR_CLOSED : STATUS_DOOR_OPEN );  
}

// -------- interrupt handlery ------------------


// watchdog handler - vyvoláno časovačem
ISR(WDT_vect) {
  sleep_disable();
  wdt_disable();  // disable watchdog
  detachInterrupt(digitalPinToInterrupt(SENSOR_PIN));
  watchdogTimer ++;
}

// interrupt vyvolaný sensorem
void ISR_sensorHandler() {
  digitalWrite(INTR_LED, HIGH);
  sleep_disable();
  wdt_disable();  // disable watchdog
  detachInterrupt(digitalPinToInterrupt(SENSOR_PIN));
  doorOpenEvent = true;
}



// ----------------------------------------------
// ---------------  S E T U P   -----------------
// ----------------------------------------------
void setup() {
  pinMode(SIGNAL_LED_PIN, OUTPUT);
  pinMode(INTR_LED, OUTPUT);
  pinMode(SIGNAL_EXT_ALARM, OUTPUT);

  digitalWrite(INTR_LED, LOW);
  digitalWrite(SIGNAL_EXT_ALARM, LOW);

  pinMode(SENSOR_PIN, INPUT_PULLUP);
  pinMode(BUTTON_PIN, INPUT_PULLUP);

  pinMode(MODEM_WAKEUP_PIN, OUTPUT);
  digitalWrite(MODEM_WAKEUP_PIN, HIGH);


  // dá příležitost ke stisku tlačítka pro zapnutí info módu
  readCheckButton();

  if (infoMode) {
    delay(1000);  // prodleva pro možnost "přečtení" morseovky na diodě
    LED_INFO();
    delay(1000);
  }

  if ( initModem()  ) {
    LED_READY();
  } else {
    LED_ERROR();
  }
  delay(500);
}



// -----------------------------------------------
// --------------   L O O P   --------------------
// -----------------------------------------------

void loop() {
  bool ok;

  if ( infoMode ) {
    digitalWrite(SIGNAL_LED_PIN, HIGH);
    delay(3);
    digitalWrite(SIGNAL_LED_PIN, LOW);
  }

  if ( !alarm ) {
    // spíme pouze v klidovém stavu
    goSleep();

    if ( doorOpenEvent ) {
      // vzbudil nás alarm
      alarm = true;

      ok = sendAlarm();

      if (infoMode) {
        if (ok) {
          LED_ALARM();
        } else {
          LED_ERROR();
        }
      }
      alarmTime = millis() + CLEAR_EXT_ALARM_TIMEOUT;
      closeTimeOK = millis() + CLOSE_DOOR_DELAY_MILLIS; // min čas, kdy bude alarm zrušen, pokud se dveře zavřou

    } else {
      // jsme probuzeni watchdogem

      // cca 1x za 24 hodin info o stavu (heartbeat)
      if ( watchdogTimer % 10536 == 0 ) {
        sendState(encodeDoorState(STATUS_HEARTBEAT) );
      }
    }

  } else {
    // jsme v ALARM modu - čekáme na uzavření dveří nebo stisk check tlačítka

    if (isDoorClosed() ) {

      if ( millis() >=  closeTimeOK ) {
        sendState( encodeDoorState(STATUS_INFO) );
        alarm = false;            // vypínáme alarm
        extAlarmOff();
      }
    } else {
      if ( digitalRead(BUTTON_PIN) == LOW ) {
        if ( !accessAllowed ) {
          if (  millis() > (alarmTime - 1000) ) {
            LED_OK();
            delay(1000);
            LED_OK();
            accessAllowed = true;
          }
        } else {
          LED_TEST();
          if ( sendState(encodeDoorState(STATUS_INFO | STATUS_HEARTBEAT ) ) ) {
            LED_OK();
          } else {
            LED_ERROR();
          }
        }
      }
      if ( !accessAllowed && millis() > alarmTime ) {
        extAlarmOn();
      }
      // jsme v alarmu, dveře stále otevřené - protáhneme čas
      closeTimeOK = millis() + CLOSE_DOOR_DELAY_MILLIS;
    }
    delay(50);
  }
}


// -------------------------------------------------

void readCheckButton() {
  digitalWrite(SIGNAL_LED_PIN, HIGH);    // signál, že nabíháme
  delay(500);

  for (byte i = 255; i > 0 && !infoMode ; i--) {
    delay(10);
    // pokud se drží test button - je aktivována LED
    infoMode |= digitalRead(BUTTON_PIN) == LOW;
  }

  digitalWrite(SIGNAL_LED_PIN, LOW);
}




void goSleep()
{
  digitalWrite(INTR_LED, LOW);
  cli();
  MCUSR = 0;                          // reset various flags
  WDTCSR |= 0b00011000;               // see docs, set WDCE, WDE
  WDTCSR =  0b01000000 | 0b100001;    // 8 sec
  wdt_reset();

  ADCSRA = 0;                             // disable ADC
  set_sleep_mode (SLEEP_MODE_PWR_DOWN);   // sleep mode is set here
  sleep_enable();

  doorOpenEvent = false;
  attachInterrupt(digitalPinToInterrupt(SENSOR_PIN), ISR_sensorHandler, LOW );
  sei();
  sleep_cpu();            // now goes to Sleep and waits for the interrupt
}




// -------------------------------------------------------
// --                    M O D E M
// -------------------------------------------------------

void clearBuffer() {
  // vycisteni bufferu, pokud tam neco je ke čtení
  while ( Serial.available() > 0 ) {
    Serial.readString();
  }
}

void modemAtCmd(char* cmd) {
  clearBuffer();
  Serial.write(cmd);
  Serial.flush();
}

bool modemSleep() {
  return sendAtCmd(AT_SLEEP);
}

String modemResponse() {
  // cekame na data a úplně se nechceme zaseknout, pokud se nedaří z modemu číst
  // odeslání dat někdy trvá dlouho
  String tmp = "";
  int cnt = 2000;

  while (true ) {
    while (true) {
      cnt--;
      delay(50);
      if ( Serial.available() > 0 || cnt == 0) {
        break;
      }
    }
    tmp += Serial.readString();
    if ( tmp.endsWith("\n") || cnt == 0 ) {
      return tmp;
    }
  }
}


bool sendAtCmd(char* cmd) {
  modemAtCmd(cmd);
  return (modemResponse() == "OK\r\n");
}

bool modemWakeUp() {
  if (!MODEM_ENABLED) {
    return true;  // debug - jako že se nám to povedlo
  }

  digitalWrite(MODEM_WAKEUP_PIN, LOW);
  delay(100);
  digitalWrite(MODEM_WAKEUP_PIN, HIGH);
  delay(100);
  return sendAtCmd(AT_CHECK);
}

// inicializuje port, překontroluje, zda odpovídá na AT příkaz
// a pošle ho do hlubokého spánku
bool initModem() {
  if (!MODEM_ENABLED) {
    return true;
  }

  Serial.begin(9600);
  if (!Serial) {
    return false;
  }

  return ( sendAtCmd(AT_CHECK) &&  modemSleep() );
}

// Alarm - sensor ohlásil otevření
bool sendAlarm() {
  if (!MODEM_ENABLED) {
    return true;  // debug - jako že jsme to poslali
  }

  if ( !modemWakeUp() ) {
    return false;
  }

  String cmd = AT_SEND_FRAME + byteToHEX( encodeDoorState(STATUS_ALARM) );
  if (EXTERNAL_ALARM) {
    cmd += ",1";   // zadost o downlink data
  } 

  cmd += "\n";

  modemAtCmd(cmd.c_str());
  String resp = modemResponse();

  bool result = resp.startsWith("OK");

  if (result && EXTERNAL_ALARM) {
    // pokud downlink vrátil první hexa 01, je přístup povolen
    // a externí alarm se nebude volat
    if ( resp.indexOf("RX=01") != -1 ) {
      accessAllowed = true;
    }
  }

  modemSleep();

  return result;
}


// stav - zjistí se stav baterie a pošle se současně
// se stavem sensoru
bool sendState(byte status) {
  if (!MODEM_ENABLED) {
    return true;  // debug - jako že jsme to poslali
  }

  if ( !modemWakeUp() ) {
    return false;
  }

  modemAtCmd(AT_TEMP);
  String temp = modemResponse();
  temp.trim();

  modemAtCmd(AT_VOLTAGE);
  String voltage = modemResponse();
  if (voltage.length() > 4) {
    voltage = voltage.substring(0, 4);
  }

  String cmd = AT_SEND_FRAME + byteToHEX(status);
  
  cmd += toHEX(temp);
  cmd += toHEX(voltage);
  cmd += "\n";

  bool result = sendAtCmd(cmd.c_str());
  modemSleep();
  return result;
}

String byteToHEX(byte b) {
    String hex = String(b, HEX);
    return (( hex.length() == 1 ? "0" : "") + hex);
}

String toHEX(String s) {
  String tmp = "";

  for (int i = 0; i < s.length(); i++ ) {
    tmp += byteToHEX(  s.charAt(i) );
  }
  return tmp;
}

// -------------------------------------------------------
void extAlarmOn() {
  if (EXTERNAL_ALARM) {
    digitalWrite(SIGNAL_EXT_ALARM, HIGH);
  }
}

void extAlarmOff() {
  digitalWrite(SIGNAL_EXT_ALARM, LOW);
}

// -------------------------------------------------------
// interpretuje pattern jako morseovku na info LED diodě
// 01 - tečka
// 10 - čárka
// 11 - mezera mezi písmeny
// 00 - -
void infoLED(unsigned long pattern) {
  byte  v;
  int d;

  while ( pattern > 0 ) {
    switch ( pattern & 0xC0000000 ) {
      case 0: d = 0;  break;
      case 0x40000000: d = 150; v = HIGH; break; // tečka
      case 0x80000000: d = 300; v = HIGH; break; // čárka
      case 0xC0000000: d = 300; v = LOW; break;  // mezera
    }
    pattern = pattern << 2 ;
    if ( d == 0 ) {
      continue;
    }

    digitalWrite(SIGNAL_LED_PIN, v);
    delay(d);
    digitalWrite(SIGNAL_LED_PIN, LOW);
    delay(150);
  }
}
