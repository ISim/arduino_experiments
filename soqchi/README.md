# Soqchi - hlídač dveří


## Popis

Program hlídá dveřní čidlo a pokud dojde k přerušení obvodu, je odeslána alarm zpráva 
pomocí SIGFOX modemu. Pokud dojde k uzavření dveří a toto uzavření trvá definovanou dobu
(nastaveno na cca 80 sekund), je odeslána info zpráva. Infor zpráva je také odeslána
každých cca 24 hodin je odeslána info zpráva, která nese informaci o stavu dveřního 
snímače, teplotě a napětí baterie.

Pro snížení spotřeby je vlastní obvod i modem většinu času ve stavu spánku a pomocí
přerušení (INTERRUPT) od watchdogu nebo čidla se probouzí je na nezbytně nutnou dobu.

Při startu se rozsvítí na 2s informační dioda. Pokud dojde je v této době  podržení
test tlačítka, je program v "info" módu:

* každých cca 8sec po probuzení ze spánku dioda krátce problikne
* pokud je úspěšně odeslán alarm modemem, vybliká dioda ALRM v morseovce

Pokud není tlačítko test strisnuto v době, kdy svítí dioda, je nastaven tichý mód 
- dioda pravidelně nebliká, alarm také není signalizován blikáním. 

I v tichém módu včask dioda signalizuje:

* po inicializaci modemu vybliká:
  * "R" - pokud je modem úspěšně inicializován a uspán
  * "ERR" - pokud se inicializace modemu nezdařila
* stisknutí tlačítka "test" vybliká "TST" 

Tlačítko "test" odesílá informační datovou zprávu - viz. dále.

## HW

Přerušení (a tím probuzení ze spánku) může být vyvoláno:

* dveřním čidlem
* tlačítkem pro test

Obě tlačítka uzemňují pin PB1, který umí vovalota přerušení. Tlačítko pro 
test ještě uzemňuje PA7.


## Datové zprávy 

Datové zprávy jsou odeslány na SIGFOX cloud.

* alarm - datová zpráva nese pouze jeden bajt s hodnotou 0xFF
* info zpráva - je odeslána po stisku tlačítka "test":
   * data[0]
      * 00 - dveře jsou zavřené
      * 01 - dveře jsou otevření
   * data[1-3] - ASCII číslice s hodnotou teploty čidla (x10)
   * data[4-9] - ASCII číslice s hodnotou napětí v mV




