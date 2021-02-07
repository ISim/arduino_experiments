#include <avr/io.h>
#include <avr/sleep.h> //Needed for sleep_mode
#include <avr/wdt.h> //Needed to enable/disable watch dog timer
#include <avr/interrupt.h>  //needed for watchdog interrupt


#define BUTTON_PIN PIN_PB1

uint32_t volatile watchdog_timer = 0;
uint32_t volatile button = 0;

void init_wdt_8s();


ISR(WDT_vect) {
  watchdog_timer ++;
}

void buttonHandler() {
  button = 1;
}


void setup() {
  pinMode(PIN_PA0, OUTPUT);
  pinMode(BUTTON_PIN, INPUT_PULLUP);
  watchdog_timer = 0;
  button = 0;
}

void loop() {

  goSleep();

  if (watchdog_timer >= 4 ) {
    watchdog_timer = 0;
    blink(1);
  }
  if ( button == 1 ) {
    blink(5);
    button = 0;
  }
}

void blink(int cnt) {
  for (; cnt > 0; cnt--) {
    digitalWrite(PIN_PA0, HIGH);
    delay(150);
    digitalWrite(PIN_PA0, LOW);
    delay(200);
  }
}


void goSleep()
{
  cli();
  wdt_reset();
  WDTCSR = _BV(WDE) | _BV(WDIE) | _BV(WDP0) | _BV(WDP3);
  attachInterrupt(digitalPinToInterrupt(BUTTON_PIN), buttonHandler, LOW);
  sleep_enable();
  set_sleep_mode(SLEEP_MODE_PWR_DOWN);
  sei();

  sleep_mode();
  cli();
  wdt_reset();
  detachInterrupt(BUTTON_PIN);
  sei();
}
