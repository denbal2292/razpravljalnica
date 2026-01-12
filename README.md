# PS 2025/2026 - Razpravljalnica

**Avtorja:** Denis Balant, Enej Hudobreznik

Projekt implementira porazdeljeno razpravljalnico, ki je sestavljena iz 3 glavnih delov:

- Nadzorne ravnine (`control/`), ki je z implementacijo protokola `Raft` robustna na izpade vozlišč znotraj nje
- Podatkovne ravnine (`server/`), v kateri so podatkovni strežniki med seboj dvosmerno povezani z verižno replikacijo
- Odjemalca (`client/`)

### 1. Nadzorna ravnina

Za delovanje aplikacije je najprej potrebno zagnati nadzorno ravnino, ki jo v osnovi sestavljajo
3 instance strežnikov in je zato z implementacijo protokola `Raft` odporna na izpad enega (za večino potrebujemo 2 glasova).

Vozlišča v nadzorni ravnini komuniciraj preko protokola `Raft`, z ostalimi pa preko gRPC, zato morajo poslušati na dveh naslovih.

IP naslovi in vrata strežnikov za so za lažji zagon vnaprej določeni:

```
node0: 127.0.0.1:7000 (Raft), 127.0.0.1:50051 (gRPC)
node1: 127.0.0.1:7001 (Raft), 127.0.0.1:50052 (gRPC)
node2: 127.0.0.1:7002 (Raft), 127.0.0.1:50053 (gRPC)
```

Nadzorno ravnino postavimo tako, da naslednje ukaze poženemo vsakega v svojem oknu terminala:

```
go run ./cmd/control --node 0 --type gui
go run ./cmd/control --node 1 --type gui
go run ./cmd/control --node 2 --type gui
```

Če si terminalskega uporabniškega vmesnika ne želimo in smo zadovoljni z manj bogatim (ampak hitrejšim in za razvoj primernejšim) izpisom, lahko zastavico `--type gui` izpustimo. Z zastavico `--node` določimo enolični identifikator strežnika, ki mora iz nabora `0, 1, 2`.

### 2. Podatkovna ravnina

Ko je nadzorna ravnina zagnana, lahko začnemo z zagonom poljubnega števila strežnikov, ki se bodo samodejno registrirali pri nadzorni ravnini, ta pa jih bo na ustrezen način povezala v verigo in jih ob morebitnem izpadu tudi prevezala.

Za vsako željeno instanco strežnika odpremo novo okno terminala in vnesemo:

```
go run ./cmd/server --type gui
```

Tudi tukaj lahko zastavico `--type gui` izpustimo. Na voljo sta še dve zastavici:

- `--port`: določimo vrata na katerih strežnik posluša. Privzeta vrednost je `0`, kar pomeni, da se nastavijo na ustrezno naključno vrednost.
- `--control-port`: podamo bazna vrata, ki jih uporabimo za izračun naslovov za dostop do strežnikov na nadzorni ravnini. Privzeta vrednost je `50051`.

### 3. Odjemalec

Ko je zaledni del pripravljen lahko poženemo enega ali več odjemalcev:

```
go run ./cmd/client --type gui
```

Dodatna navodila za uporabo odjemalca so na ob zagonu.

Zastavico `--type gui` lahko seveda tudi tukaj izpustimo, a tega za dobro uporabniško izkušnjo ne priporočava. Tudi odjemalec sprejme zastavico `--control-port`, ki ima enak pomen kot pri zagonu strežnika v podatkovni ravnini.

Ko se odjemalec poveže na trenutnega voditelja v kontrolni ravnini, mu ta sporoči naslov trenutne glave in repa verige strežnikov v podatkovni ravnini. Vsa pisanja
se samodejno izvajajo na glavo, vsa branja pa na rep. V primeru, da se stanje verige spremeni in odjemec v roku ne dobi ustreznega, odgovora pridobi novo stanje verige iz nadzorne ravnine.

Podobno delujejo naročnine na teme, kjer si z replikacijo podatkovnih strežnikov lahko v primeru, da je teh dovolj, privoščimo tudi delitev obremenitve med njimi.

### Opomba:

Če želimo spremeniti vmesnik aplikacije moramo ponovno prevesti `proto` datoteko z ukazom `make proto`
