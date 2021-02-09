#include <avr/io.h>
#include <avr/sleep.h> //Needed for sleep_mode
#include <avr/wdt.h> //Needed to enable/disable watch dog timer
#include <avr/interrupt.h>  //needed for watchdog interrupt


#define RATE_LIMIT_MSGS       50        // max. počet zpráv za cca 24 hodin
#define CLOSE_TICKS           20        // počet tiků po zavření dveří, než se smaže poplach (x 8s cca)
#define RATE_LIMIT_TICKS      10        // min. počet tiků mezi odeslání zpráv

#define SENSOR_PIN        PIN_PB1   // pin pro interrupt       
#define SIGNAL_LED_PIN    PIN_PA0   // signalizační dioda
#define MODEM_WAKEUP_PIN  PIN_PB0   // reset modemu  
#define BUTTON_PIN        PIN_PA7   // info tlačítko   


#define AT_SLEEP        "AT$P=2\n"
#define AT_CHECK        "AT\n"
#define AT_OUT_OF_BAND  "AT$SO\n"
#define AT_SEND_FRAME   "AT$SF="
#define AT_TEMP         "AT$T?\n"
#define AT_VOLTAGE      "AT$V?\n"

#define LED_OK()      infoLED(0b10101011100110)        // OK
#define LED_READY()   infoLED(0b011001)                // R
#define LED_ALARM()   infoLED(0b01101101100101111010)  // ALRM          
#define LED_ERROR()   infoLED(0b011101100111011001)    // ERR
#define LED_TEST()    infoLED(0b10110101011110)        // TST

uint32_t volatile watchdogTimer = 10;         // tikáček - 10 je nastaveno, aby se neblokovalo odeslání po startu
bool volatile     doorOpenEvent = false;      // do true přepne otevření dveří
bool volatile     checkEvent    = false;      // signalizace stisku info tlačítka

int               msgsRemaining = RATE_LIMIT_MSGS;  // zbývající zprávy
uint32_t          lastMessageAt = 0;                // "čas" odeslání zprávy (ve skutečnosti hodnota watchDog timeru)
bool              alarm = false;                    // stav "poplachu"
int               closeClock;                       // odpočítává dobu zavření pro vynulování poplachu
bool              ledActive;                        // pokud se při startu drží tlačítko "info", zapiná se "debug" režim

// -------- interrupt handlery ------------------
ISR(WDT_vect) {
  sleep_disable();
  wdt_disable();  // disable watchdog
  detachInterrupt(digitalPinToInterrupt(SENSOR_PIN));
  watchdogTimer ++;
}

void sensorHandler() {
  // buď reset samotný nebo "check" tlačítko
  if ( digitalRead(BUTTON_PIN) == LOW ) {
    checkEvent = true;
  } else {
    doorOpenEvent = true;
  }

  detachInterrupt(digitalPinToInterrupt(SENSOR_PIN));
  sleep_disable();
  wdt_disable();  // disable watchdog
}

bool isDoorClosed() {
  return digitalRead(SENSOR_PIN) == HIGH;
}

// ----------------------------------------------
// ---------------  S E T U P   -----------------
// ----------------------------------------------
void setup() {
  pinMode(SIGNAL_LED_PIN, OUTPUT);
  pinMode(SENSOR_PIN, INPUT_PULLUP);
  pinMode(BUTTON_PIN, INPUT_PULLUP);
  pinMode(MODEM_WAKEUP_PIN, OUTPUT);
  digitalWrite(MODEM_WAKEUP_PIN, HIGH);

  digitalWrite(SIGNAL_LED_PIN, HIGH);    // vstup do setupu
  for (byte i = 255; i>0; i--) {
    delay(10);
    // pokud se drží test button - je aktivována LED
    ledActive |= digitalRead(BUTTON_PIN) == LOW;
  }  
  digitalWrite(SIGNAL_LED_PIN, LOW);
  delay(1000);
 
  if ( initModem()  ) {
    LED_READY();
  } else {
    LED_ERROR();
  }
}



// -----------------------------------------------
// --------------   L O O P   --------------------
// -----------------------------------------------

void loop() {
  bool ok;

  if ( ledActive ) {
    digitalWrite(SIGNAL_LED_PIN, HIGH);
    delay(5);
    digitalWrite(SIGNAL_LED_PIN, LOW);
  }


  goSleep();

  if ( checkEvent ) {
    // stisknuto "check" tlačítko
    LED_TEST();
    if ( sendState(isDoorClosed()) ) {
      LED_OK();
    } else {
      LED_ERROR();
    }

  } else if ( doorOpenEvent ) {
    if ( !alarm ) {
      alarm = true;
      closeClock = CLOSE_TICKS;
      ok = sendAlarm();
      if (ledActive) {
        if (ok) {
          LED_ALARM();
        } else {
          LED_ERROR();
        }
      }
    }
  } else {
    // probuzení watchdogem

    // cca 1x 24 hod vynulovat čítač zpráv
    if ( watchdogTimer % 10800 == 0 ) {
      msgsRemaining = RATE_LIMIT_MSGS;
    }

    // cca 1x za 48 hodin info o stavu
    if ( watchdogTimer % 21600 == 0 ) {
      sendState(isDoorClosed());
    }

    if (alarm && isDoorClosed() ) {
      closeClock--;
      if ( closeClock == 0 ) {
        sendState(isDoorClosed());
        alarm = false;            // odblokování
      }
    }
  }
}

// -------------------------------------------------

void goSleep()
{
  cli();
  MCUSR = 0;                          // reset various flags
  WDTCSR |= 0b00011000;               // see docs, set WDCE, WDE
  WDTCSR =  0b01000000 | 0b100001;    // 8 sec
  wdt_reset();

  byte adcsra_save = ADCSRA;
  ADCSRA = 0;                              // disable ADC
  set_sleep_mode (SLEEP_MODE_PWR_DOWN);   // sleep mode is set here
  sleep_enable();

  doorOpenEvent = false;
  checkEvent = false;
  attachInterrupt(digitalPinToInterrupt(SENSOR_PIN), sensorHandler, LOW );

  sei();

  sleep_cpu ();            // now goes to Sleep and waits for the interrupt

  ADCSRA = adcsra_save;  // stop power reduction
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
  digitalWrite(MODEM_WAKEUP_PIN, LOW);
  delay(100);
  digitalWrite(MODEM_WAKEUP_PIN, HIGH);
  delay(100);
  return sendAtCmd(AT_CHECK);
}


bool initModem() {
  Serial.begin(9600);

  return ( sendAtCmd(AT_CHECK) &&  modemSleep() );
}


// posoudi, zda není frekvence zpráv příliš vysoká (např. kvůli chybě, flatterování snímače,...)
// a zda jsou k dispozici volné zprávy
bool canBeSent() {
  if ( (watchdogTimer - lastMessageAt) < RATE_LIMIT_TICKS ) {
    return false;
  }
  if (msgsRemaining > 0) {
    msgsRemaining--;
    return true;
  }
  return false;
}


// Alarm - sensor ohlásil otevření
bool sendAlarm() {
  if ( !canBeSent() ) {
    return false;
  }

  if ( !modemWakeUp() ) {
    return false;
  }
  String cmd = AT_SEND_FRAME;
  cmd += "FF\n";

  bool result = sendAtCmd(cmd.c_str());
  modemSleep();
  lastMessageAt = watchdogTimer;

  return result;
}


// stav - zjistí se stav baterie a pošle se současně
// se stavem sensoru - funkce nenastavuje lastMessageAt, aby odeslání
// stavové zprávy nevyblokovalo alarm
bool sendState(bool closed) {

  if ( !canBeSent() ) {
    return false;
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
    voltage = voltage.substring(1, 4);
  }

  String cmd = AT_SEND_FRAME;
  if (closed) {
    cmd += "00";
  } else {
    cmd += "01";
  }
  cmd += toHEX(temp);
  cmd += toHEX(voltage);
  cmd += "\n";

  bool result = sendAtCmd(cmd.c_str());
  modemSleep();
  return result;
}

String toHEX(String s) {
  String tmp = "";
  String hex;

  for (int i = 0; i < s.length(); i++ ) {
    hex = String(byte(s.charAt(i)), HEX);
    tmp += (( hex.length() == 1 ? "0" : "") + hex);
  }
  return tmp;
}

// -------------------------------------------------------
// interpretuje pattern jako morseovku
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
      case 0xC0000000: d = 300; v = LOW; break; // mezera
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
