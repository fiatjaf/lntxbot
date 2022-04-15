package t

var DE = map[Key]string{
	NO:         "Nein",
	YES:        "Ja",
	CANCEL:     "Storniere(n)",
	CANCELED:   "Storniert.",
	COMPLETED:  "Erledigt!",
	CONFIRM:    "Bestätigen",
	PAYAMOUNT:  `Zahle Betrag an Sats {{.Sats | printf "%.15g"}}`,
	FAILURE:    "Fehlschlag.",
	PROCESSING: "Wird verarbeitet...",
	WITHDRAW:   "Sats abheben?",
	ERROR:      "🔴 {{if .App}}#{{.App | lower}} {{end}}Error{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:   "Prüfend...",
	TXPENDING:  "Zahlung noch unterwegs, bitte später erneut prüfen.",
	TXCANCELED: "Transaktion storniert.",
	UNEXPECTED: "Unerwarteter Fehler: bitte melden.",

	CALLBACKWINNER:  "Gewinner: {{.Winner}}",
	CALLBACKERROR:   "{{.BotOp}} error{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED: "{{.BotOp}} abgelaufen.",
	CALLBACKATTEMPT: "Zahlungsversuch. /tx_{{.Hash}}",
	CALLBACKSENDING: "Zahlungssendung.",

	INLINEINVOICERESULT:  "Zahlungsanfrage für {{.Sats}} X Sats.",
	INLINEGIVEAWAYRESULT: "Verschenke {{.Sats}} Sats {{if .Receiver}}an @{{.Receiver}}{{else}}her{{end}}",
	INLINEGIVEFLIPRESULT: "Verschenke {{.Sats}} Sats an einen von {{.MaxPlayers}} X Teilnehmern",
	INLINECOINFLIPRESULT: "Lotterie mit einem Eintrittspreis von {{.Sats}} Sats für maximal {{.MaxPlayers}} Teilnehmer",
	INLINEHIDDENRESULT:   "{{.HiddenId}} ({{if gt .Message.Crowdfund 1}}crowd:{{.Message.Crowdfund}}{{else if gt .Message.Times 0}}priv:{{.Message.Times}}{{else if .Message.Public}}pub{{else}}priv{{end}}): {{.Message.Content}}",

	LNURLUNSUPPORTED: "Diese Art von lnurl wird hier nicht unterstützt.",
	LNURLERROR:       `<b>{{.Host}}</b> lnurl error: {{.Reason}}`,
	LNURLAUTHSUCCESS: `
lnurl-auth success!

<b>Domain</b>: <i>{{.Host}}</i>
<b>Public Key</b>: <i>{{.PublicKey}}</i>
`,
	LNURLPAYPROMPT: `🟢 <code>{{.Domain}}</code> erwartet {{if .FixedAmount}}<i>{{.FixedAmount | printf "%.15g"}} sat</i>{{else}} einen Wert zwischen <i>{{.Min | printf "%.15g"}}</i> und <i>{{.Max | printf "%.15g"}} sat</i>{{end}} for:

<code>{{if .Long}}{{.Long | html}}{{else}}{{.Text | html}}{{end}}</code>{{if .WillSendPayerData}}

---

- Dein Name und/oder Authentifizierungsschlüssel wird an den Zahlungsempfänger gesendet.
- Um das zu verhindern, benutze <code>/lnurl --anonymous &lt;lnurl&gt;</code>.
{{end}}

{{if not .FixedAmount}}<b>Antworte mit der Anzahl (in satoshis, zwischen <i>{{.Min | printf "%.15g"}}</i> und <i>{{.Max | printf "%.15g"}}</i>) um zu bestätigen.</b>{{end}}
    `,
	LNURLPAYPROMPTCOMMENT: `📨 <code>{{.Domain}}</code> erwartet einen Kommentar.

<b>Um die Zahlung zu bestätigen, bitte mit einem Text antworten</b>`,
	LNURLPAYAMOUNTSNOTICE: `<code>{{.Domain}}</code> erwartet {{if .Exact}}{{.Min | printf "%.3f"}}{{else if .NoMax}} mindestens {{.Min | printf "%.0f"}}{{else}} zwischen {{.Min | printf "%.0f"}} und {{.Max | printf "%.0f"}}{{end}} sat.`,
	LNURLPAYSUCCESS: `<code>{{.Domain}}</code> sagt:
{{.Text}}
{{if .DecipherError}}Fehler beim Entziffern ({{.DecipherError}}):
{{end}}{{if .Value}}<pre>{{.Value}}</pre>
{{end}}{{if .URL}}<a href="{{.URL}}">{{.URL}}</a>{{end}}
    `,
	LNURLPAYMETADATA: `#lnurlpay metadata:
<b>domain</b>: <i>{{.Domain}}</i>
<b>transaction</b>: /tx_{{.HashFirstChars}}
    `,
	LNURLBALANCECHECKCANCELED: "Automatische Kontostandsprüfungen von {{.Service}} werden storniert.",

	TICKETSET: "Neue Gruppenmitglieder müssen einen Betrag/eine Rechnung von X Sats bezahlen {{.Sat}} (Vergewissere dich, dass du dafür @lntxbot als Administrator festgelegt hast).",

	RENAMABLEMSG:      "Jeder kann diese Gruppe umbenennen, wenn er Betrag X an Sats bezahlt {{.Sat}} (Vergewissere dich, dass du @lntxbot als Administrator festgelegt hast).",
	RENAMEPROMPT:      "Bezahle <b>{{.Sats}} sat</b> um diese Gruppe umzubennenen <i>{{.Name}}</i>?",
	GROUPNOTRENAMABLE: "Diese Gruppe kann nicht umbenannt werden!",

	INTERNALPAYMENTUNEXPECTED: "Etwas Unerwartetes ist passiert. Wenn das eine interne Rechnung ist, wird sie fehlschlagen. Vielleicht ist die Rechnung abgelaufen oder etwas anderes ist passiert, wir wissen es nicht. Wenn das eine externe Rechnung ist, ignoriere die Warnung.",
	PAYMENTFAILED:             "❌ Bezahlung <code>{{.Hash}}</code> fehlgeschlagen.\n\n<i>{{.FailureString}}</i>",
	PAIDMESSAGE: `✅ Paid with <i>{{printf "%.15g" .Sats}} sat</i> ({{dollar .Sats}}) (+ <i>{{.Fee}}</i> fee). 

<b>Hash:</b> <code>{{.Hash}}</code>{{if .Preimage}}
<b>Proof:</b> <code>{{.Preimage}}</code>{{end}}

/tx_{{.ShortHash}} ⚡️ #tx`,
	OVERQUOTA:           "Du hast dein {{.App}} Wochenkontingent überschritten/ausgeschöpft.",
	RATELIMIT:           "Diese Aktion ist geschwindigkeitsbegrenzt. Bitte warte 30 Minuten.",
	DBERROR:             "Datenbankfehler: konnte die Transaktion nicht als nicht-ausstehend markieren.",
	INSUFFICIENTBALANCE: `Unzureichendes Guthaben für {{.Purpose}}. Benötigt {{.Sats | printf "%.15g"}} Sats mehr.`,

	PAYMENTRECEIVED: `
      ⚡️ Zahlung erhalten{{if .SenderName}} von <i>{{ .SenderName }}</i>{{end}}: {{.Sats}} sat ({{dollar .Sats}}). /tx_{{.Hash}}{{if .Message}} {{.Message | messageLink}}{{end}} #tx
      {{if .Comment}}
📨 <i>{{.Comment}}</i>
      {{end}}
    `,
	FAILEDTOSAVERECEIVED: "Bezahlung erhalten, aber die Speicherung in der Datenbank ist gescheitert. Bitte melde das Problem: <code>{{.Hash}}</code>",

	SPAMMYMSG:             "{{if .Spammy}}Diese Gruppe ist jetzt spammy.{{else}}Spamming beendet.{{end}}",
	COINFLIPSENABLEDMSG:   "Coinflips (Münzwürfe) sind dieser Gruppe aktiviert {{if .Enabled}}aktiviert{{else}}deaktiviert{{end}} .",
	LANGUAGEMSG:           "Die Chatsprache ist auf folgende Sprache eingestellt <code>{{.Language}}</code>.",
	FREEJOIN:              "Dieser Gruppe kann nun beigetreten werden.",
	EXPENSIVEMSG:          "Jede Nachricht in dieser Gruppe{{with .Pattern}} die dieses Muster/Inhalt enthält <code>{{.}}</code>{{end}} kostet {{.Price}} Sats.",
	EXPENSIVENOTIFICATION: "Die Nachricht {{.Link}} hat {{if .Sender}} dich gerade {{.Price}} gekostet{{else}} dir {{.Price}} gebracht {{end}}.",
	FREETALK:              "Nachrichten sind wieder kostenlos",

	APPBALANCE: `#{{.App | lower}} Balance: <i>{{printf "%.15g" .Balance}} sat</i>`,

	HELPINTRO: `
<pre>{{.Help}}</pre>
Für mehr Informationen zu jedem Befehlstypen <code>/help &lt;command&gt;</code>.
    `,
	HELPSIMILAR: "/{{.Method}} Befehl nicht gefunden. Meinst Du /{{index .Similar 0}}?{{if gt (len .Similar) 1}} Oder vielleicht /{{index .Similar 1}}?{{if gt (len .Similar) 2}} Vielleicht /{{index .Similar 2}}?{{end}}{{end}}",
	HELPMETHOD: `
<pre>/{{.MainName}} {{.Argstr}}</pre>
{{.Help}}
{{if .HasInline}}
<b>Inline query</b>
Kann auch als <a href="https://core.telegram.org/bots/inline">inline query</a> von einer Gruppe oder Person, von denen der Bot nicht hinzugefügt wurde aufgerufen werden . Die Syntax ist ähnlich, aber vereinfacht: <code>@{{.ServiceId}} {{.InlineExample}}</code> dann warten bis es als "search result" (Suchergebnis) erscheint.{{end}}
{{if .Aliases}}
<b>Aliases:</b> <code>{{.Aliases}}</code>{{end}}
    `,

	// das "any" (irgendwas) wird hier nur zu Demonstrationszwecken verwendet. Wenn du das 'any' verwendest, wird es
	// tatsächlich mit <satoshis> Variablen verknüpft, denn so löst das der Code.
	RECEIVEHELP: `Generiert eine BOLT11 Rechnung mit einem vorgegebenen Satoshi Wert. Der Betrag wird @lntxbot Guthaben gut geschrieben. Wenn Du keinen Betrag einträgst, wird eine open-ended Rechnung generiert, die mit jedem Betrag beglichen werden kann.",

<code>/receive_320_for_something</code> generiert eine Rechnung in Höhe 320 Sats mit der Beschreibung "für etwas"
    `,

	PAYHELP: `Dekodiert eine BOLT11 Rechnung und fragt, ob du sie begleichen willst (wenn nicht /jetzt bezahlen). Das ist der gleiche Vorgang, als ob du eine Rechnung im Chat einfügen oder weiterleiten würdest. Genauso funktioniert die Verwendung eines Bildes mit QR Code, in dem die Rechnung enthalten ist (wenn das Bild scharf ist).

Einfach <code>lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> einfügen dann wird die Rechnung dekodiert sie und das Programm bittet um Zahlung der Rechnung.  

<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> zahlt den Betrag ohne um Bestätigung zu fragen.

/withdraw_lnurl_3000 generiert eine <b>lnurl und einen QR code um 3000</b> satoshis von einer <a href="https://lightning-wallet.com">kompatiblen Wallet</a> abzuheben ohne auf Betätigung zu warten.
    `,

	SENDHELP: `Sende Satoshis zu anderen Telegram Nutzern. Der Empfänger erhält in seinem Chat eine Benachrichtigung von @lntxbot. Wenn der Empfänger niemals mit dem Bot kommuniziert hat oder diesen geblockt hat, kann er nicht benachrichtigt werden. In diesem Fall kannst du die Transaktion nachträglich in der Transaktionsansicht stornieren.

<code>/tip 100</code>, wenn dies in einer Gruppe, in der der Bot installiert ist, als Antwort auf eine Nachricht gesendet wird, werden 100 Satoshis an den Autor der Nachricht gesendet.
<code>/send 500 @username</code> sendet 500 Satoshis an den Telegram Nutzer @username.
<code>/send anonymously 1000 @someone</code> das Gleiche wie oben, aber der Telegram Nutzer @someone wird nur sehen: "Jemand hat dir 1000 Satoshis gesendet".
    `,

	TRANSACTIONSHELP: `
Listet alle Transaktionen mit Seitennmmerierung (pagination controls). Jede Transaktion ´verfügt über einen Link, der für mehr Informationen angeklickt werden kann. 

/transactions listet alle Transaktionen auf, angefangen mit den Neuesten.
<code>/transactions --in</code> listet nur eingehende Transaktionen auf.
<code>/transactions --out</code> listet nur ausgehende Transactions auf.
    `,

	BALANCEHELP: "zeigt das Guthaben in Satoshis und zusätzlich die Summe von allem, was du empfangen und mit dem Bot gesendet hast sowie der Gesamtbetrag an Gebühren.",

	GIVEAWAYHELP: `Erstellt einen Button in einem Gruppenchat. Der erste Nutzer, der darauf klickt, erhält die Satoshis.

/giveaway_1000: wenn jemand den "Beanspruchen Button" anklickt, werden 1000 Satoshis von dir zu dieser Person transferiert. 
    `,
	SATSGIVENPUBLIC: "{{.Sats}} sats gegeben von {{.From}} an {{.To}}.{{if .ClaimerHasNoChat}} Um Dein Guthaben zu managen/bearbeiten, starte eine Konversation mit @lntxbot.{{end}}",
	CLAIMFAILED:     "Beanspruchen (Claim) fehlgeschlagen {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:   "Beanspruchen",
	GIVEAWAYMSG:     "{{.User}} Nutzer {{if .Away}}verschenkt{{else if .Receiver}}@{{.Receiver}}{{else}}gibt dir{{end}} {{.Sats}} sats!",

	COINFLIPHELP: `Startet eine gerechte/faire Lotterie mit einer angegebenen Zahl an Teilnehmern. Jeder zahlt das gleiche Eintrittsgeld. Der Gewinner erhält alle Beiträge. Die Geldmittel werden nur von den Teilnehmerkonten bewegt, wenn die Lotterie verwirklicht/erfüllt wird.

/coinflip_100_5: 5 Teilnehmer benötigt, der Gewinner erhält 500 Satoshis (inklusive seiner eingesetzten 100, so dass es Netto 400 Satoshis sind).
    `,
	COINFLIPWINNERMSG:      "Du bist der Gewinner eines Münzwurfes mit einem Preisgeld von {{.TotalSats}} sat. Die Verlierer sind: {{.Senders}}.",
	COINFLIPGIVERMSG:       "Du hast {{.IndividualSats}} bei einem Münzwurf verloren. Der Gewinner ist {{.Receiver}}.",
	COINFLIPAD:             "Zahle {{.Sats}} Satoshi und habe die Chance, {{.Prize}} Satoshi zu gewinnen! {{.SpotsLeft}} von {{.MaxPlayers}} Plätzen {{s .SpotsLeft}} übrig!",
	COINFLIPJOIN:           "Nehme an der Lotterie teil!",
	CALLBACKCOINFLIPWINNER: "Münzwurf Gewinner: {{.Winner}}",

	GIVEFLIPHELP: `Startet ein Geschenk, aber anstatt es der ersten Person die klickt zu geben, wird der Betrag zwischen den ersten x Teilnehmern verlost. 

/giveflip_100_5: 5 Teilnehmer benötigt, der Gewinner erhält 500 Satoshis vom Initiator/Befehlsgeber.
    `,
	GIVEFLIPMSG:       "{{.User}} Nutzer gibt {{.Sats}} Sats weg an eine glückliche Person aus X Teilnehmern {{.Participants}}!",
	GIVEFLIPAD:        "{{.Sats}} werden verschenkt. Nimm teil und nutze die Möglichkeit zu gewinnen! {{.SpotsLeft}} Plätze von {{.MaxPlayers}} Plätzen verfügbar!",
	GIVEFLIPJOIN:      "Versuche zu gewinnen!",
	GIVEFLIPWINNERMSG: "{{.Sender}} hat an {{.Receiver}} {{.Sats}} Sats gesendet. Diese Personen haben nicht bekommen: {{.Losers}}.{{if .ReceiverHasNoChat}} Um Dein Guthaben zu managen/bearbeiten, starte eine Konversation with @lntxbot.{{end}}",

	FUNDRAISEHELP: `Starte ein Crowdfunding mit einer festgelegten Zahl an Teilnehmern und Spendenbetrag. Wenn die vorgegebene Zahl an Teilnehmern erreicht wird, wird es angewendet. Ansonsten wird es nach einigen Stunden storniert.

<code>/fundraise 10000 8 @user</code>: Telegram Nutzer @user wird 8000 Satoshis erhalten, nachdem 8 Personen teilgenommen/beigtragen haben. 
    `,
	FUNDRAISEAD: `
Spendenaktion {{.Fund}} für {{.ToUser}}!
Zahl Spender für Vollendung benötigt: {{.Participants}}
Jeder zahlt Betrag: {{.Sats}} sat
Folgende Personen haben beigesteuert: {{.Registered}}
    `,
	FUNDRAISEJOIN:        "Spende!",
	FUNDRAISECOMPLETE:    "Spendenaktion für {{.Receiver}} abgeschlossen!",
	FUNDRAISERECEIVERMSG: "Du hast folgenden Betrag {{.TotalSats}} Sats einer Spendenaktion von dem Sender XY erhalten {{.Senders}}s",
	FUNDRAISEGIVERMSG:    "Du hast folgenden Betrag {{.IndividualSats}} in einer Spendenaktion an Person XY gegeben {{.Receiver}}.",

	LIGHTNINGATMHELP: `Gibt die Zugangsdaten/credentials im Format zurück wie von @Z1isenough's erwartet <a href="https://docs.lightningatm.me">LightningATM</a>.

Für eine spezifische Dokumention zum Aufsetzen mit dem @lntxbot besuche <a href="https://docs.lightningatm.me/lightningatm-setup/wallet-setup/lntxbot">the lntxbot setup tutorial</a> (there's also <a href="https://docs.lightningatm.me/faq-and-common-problems/wallet-communication#talking-to-an-api-in-practice">a more detailed and technical background</a>).
  `,
	BLUEWALLETHELP: `Gibt deine Zugangsdaten für den Import deiner Bot-Wallet zur BlueWallet zurück. Du kannst den gleichen Zugang für beide Accounts nutzen.

/bluewallet druckt einen String/eine Zeichenkette wie bspw. "lndhub://&lt;login&gt;:&lt;password&gt;@&lt;url&gt;" die kopiert werden muss und im Bluewallet Import Bildschirm eingefügt werden muss.
/bluewallet_refresh löscht dein vorheriges Passwort und druckt einen neuen String/eine neue Zeichenkette. Danach musst die die Zugangsdaten in der BlueWallet wieder einfügen. Mache dies nur, wenn deine vorherigen Zugangsdaten kompromittiert waren.
    `,
	APIPASSWORDUPDATEERROR: "Fehler beim Updaten des Passworts. Problem bitte melden: {{.Err}}",
	APICREDENTIALS: `
Dies sind tokens für <i>Basic Auth</i>. Die API ist über einige Umwege mit lndhub.io kompatibel.

Voller Zugang: <code>{{.Full}}</code>
Zugang zu Rechnungen: <code>{{.Invoice}}</code>
Read-Only Zugang/Nur Leserechte: <code>{{.ReadOnly}}</code>
API Base URL: <code>{{.ServiceURL}}/</code>

/api_full, /api_invoice und /api_readonly zeigt diese spezifischen Tokens zusammen mit QR Codes.
/api_url wird einen QR Code für die API Base URL.

Halte diese Tokens geheim. Wenn sie aus irgendwelchen Gründen geleakt werden, verwende /api_refresh , um sie alle zu ersetzen.
    `,

	HIDEHELP: `Versteckt eine Nachricht, die später gegen Bezahlung sichtbar gemacht werden kann.
<code>/hide 500 'teaser showed on prompt'</code>, sende diese Antwort auf irgendeine Nachricht, die Video, Audio, Bilder oder Text beinhaltet und es wird hinter einer Bezahlschranke von 500 Satoshis landen.

Modifiers:
  <code>--crowdfund &lt;number&gt;</code> ermöglicht öffentliches Crowdfunding von/über versteckte(n) Botschaften.
  <code>--private</code> deckt die versteckte Nachricht privat und nur gegenüber dem Zahlenden statt der kompletten Gruppe auf.
  <code>--revealers &lt;number&gt;</code> erlaubt lediglich eine bestimmten Anzahl an <code>&lt;number&gt;</code> Teilnehmern die versteckte Nachricht zu sehen, dann läuft die Anforderung ab.
    `,
	REVEALHELP: `Macht eine Nachricht sichtbar, die vorher versteckt war. Der Autor der versteckten Nachricht wird niemals preis gegeben. Wenn eine Nachricht versteckt ist, kann diese global/von allen aufgedeckt werden, aber nur von denen, die die versteckte ID kennen.

Eine Anforderung zur Enthüllung kann in einer Gruppe oder einem Chat auch kreiert werden, indem der "share" Button geklickt wird nachdem man die Nachricht versteckt hat. Dan werden die Standards zum Aufdecken von Nachrichten angewendet. Mehr mehr Info gehe zu /help_hide .

<code>/reveal 5c0b2rh4x</code> kreiert eine Aufforderung die folgende Nachricht zu enthüllen 5c0b2rh4x, wenn sie existiert.
    `,
	HIDDENREVEALBUTTON:   `{{.Sats}} sat to reveal {{if .Public}}in-place{{else}}privately{{end}}. {{if gt .Crowdfund 1}}{{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Left: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "Hier ist eine Nachricht versteckt. {{.Sats}} X Sats benötigt, um diese zu aufzudecken.",
	HIDDENWITHID: `Versteckte Nachricht mit einer ID <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Wird öffentlicht sichtbar sobald {{.Message.Crowdfund}} Personen zahlen {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Wird nur privat sichtbar gemacht {{.Message.Times}} gegenüber ersten Zahlenden{{else if .Message.Public}}Wird öffentlich sichtbar, sobald eine Person zahlt {{.Message.Satoshis}}{{else}}Wird gegenüber irgendeinem Zahlenden privat sichtbar gemacht{{end}}.

{{if .WithInstructions}}Verwende /reveal_{{.HiddenId}} in einer Gruppe um dort zu teilen.{{end}}
    `,
	HIDDENSOURCEMSG:   "Versteckte Nachricht <code>{{.Id}}</code> von XY enthüllt {{.Revealers}}. Du bekommst {{.Sats}} Sats.",
	HIDDENREVEALMSG:   "{{.Sats}} Betrag an Sats wurden bezahlt, um die Nachricht aufzudecken <code>{{.Id}}</code>.",
	HIDDENMSGNOTFOUND: "Versteckte Nachricht nicht gefunden.",
	HIDDENSHAREBTN:    "Teile in einem anderen Chat",

	TOGGLEHELP: `Schaltet Bot Funktionen in der Gruppe ein/aus. In Supergruppen kann es nur von Admins bedient werden.

/toggle_ticket_10 beginnt für alle neuen Gruppeneintritte Gebühren zu erheben. Als Anti-Spamfunktion nützlich. Das Geld geht an den Gruppenbesitzer.
/toggle_ticket stoppt die Gebühr für neue Gruppeneintritte. 
/toggle_language_ru ändert die Chatsprache in russisch, /toggle_language zeigt die Chatsprache an, das funktioniert auch in privaten Chats.
/toggle_spammy schaltet 'spammy' Modus ein. 'spammy' Modus ist standardmäßig deaktiviert. Wenn dies eingeschaltet wird, werden Benachrichtigungen zu Trinkgeldern in der Gruppe öffentlich angezeigt, statt privat übermittelt.
    `,

	SATS4ADSHELP: `
Sats4ads ist ein Anzeigen Marketplace auf Telegram. Zahle Geld, um anderen Personen Anzeigen zu zeigen und erhalte Geld für jede Anzeige, die du siehst.

Die Preise für jeden Nutzer sind in msatoshi-pro Zeichen. Der maximale Preis sind 1000 msat.
Jede Anzeige beinhaltet eine festgelegte Gebühr von 1 Sat.
Bilder und Videos werden so behandelt als hätten sie 100 Zeichen.
Bei Links werden zusätzlich 300 Zeichen berechnet, weil sie über eine ärgerliche Voransicht verfügen.

Um eine Anzeige zu übertragen, musst du an den Bot eine Nachricht mit dem Anzeigeninhalt senden, und dann wie beschrieben so antworten <code>/sats4ads broadcast ...</code> . Du kannst den Code <code>--max-rate=500</code> und den Code <code>--skip=0</code> nutzen, um eine bessere Kontrolle zu haben wie die Anzeige veröffentlich wird. Das sind Standardeinstellungen.

/sats4ads_on_15 versetzt deinen Account in den Status, dass Anzeigen hinzugefügt werden. Jeder wird in der Lage sein Dir für 15 msatoshi-pro-Zeichen Nachrichten zu übermitteln. Du kannst diesen Preis anpassen.
/sats4ads_off schaltet deinen Account ab, so dass du keine weiteren Anzeigen mehr bekommst.
/sats4ads_rates zeigt eine Übersicht wie viele Knotenpunkte auf jedem Preislevel existieren. Nützlich, um sein Budget/Guthaben frühzeitig zu planen.
/sats4ads_rate zeigt deinen Preis/deine Rate an.
/sats4ads_preview zeigt als Antwort auf eine Nachricht an wie viele andere Nutzer es sehen werden. Der in der Voransicht angezeigte Satoshi Betrag hat keinerlei Bedeutung.
/sats4ads_broadcast_1000 veröffentlicht eine Anzeige. Die letzte Zahl ist die maximale Zahl an Satoshis, die verwendet werden. Günstigere Anzeigenlsitings werden gegenüber teuren Listings bevorzugt. Muss als Antwort auf eine andere Nachricht aufgrufen werden deren Inhalt als Anzeigentext verwendet wird.
    `,
	SATS4ADSTOGGLE:    `#sats4ads {{if .On}}Sehe Anzeigen/Werbung und erhalte {{printf "%.15g" .Sats}} X Sats pro Zeichen.{{else}}Du wirst keine weiteren Anzeigen sehen.{{end}}`,
	SATS4ADSBROADCAST: `#sats4ads {{if .NSent}}Nachricht veröffentlicht {{.NSent}} Zeit{{s .NSent}} für Gesamtkosten in Höhe von {{.Sats}} Sats ({{dollar .Sats}}).{{else}} Konnte keinen zu benachrichtigenden Endpunkt im Netzwerk finden, um ihn über die festgelegten Parameter zu informieren. /sats4ads_rates{{end}}`,
	SATS4ADSSTART:     `Nachricht wird veröffentlicht.`,
	SATS4ADSPRICETABLE: `#sats4ads Quantity of users <b>up to</b> each pricing tier.
{{range .Rates}}<code>{{.UpToRate}} msat</code>: <i>{{.NUsers}} user{{s .NUsers}}</i>
{{else}}
<i>No one is registered to see ads yet.</i>
{{end}}
Jede Anzeige kostet den oben stehenden Preis <i>per character</i> + <code>1 sat</code> füt jeden Nutzer.
    `,
	SATS4ADSADFOOTER: `[#sats4ads: {{printf "%.15g" .Sats}} sat]`,
	SATS4ADSVIEWED:   `Claim`,

	HELPHELP: "Zeigt die komplette Hilfe oder Hilfe zu einem spezifischen Befehl.",

	STOPHELP: "Der Bot stoppt das Anzeigen von Informationen.",

	PAYPROMPT: `
{{if .Sats}}<i>{{.Sats}} sat</i> ({{dollar .Sats}})
{{end}}{{if .Description}}<i>{{.Description}}</i>{{else}}<code>{{.DescriptionHash}}</code>{{end}}
{{if .ReceiverName}}
<b>Receiver</b>: {{.ReceiverName}}{{end}}
<b>Hash</b>: <code>{{.Hash}}</code>{{if ne .Currency "bc"}}
<b>Chain</b>: {{.Currency}}{{end}}
<b>Created at</b>: {{.Created}}
<b>Expires at</b>: {{.Expiry}}{{if .Expired}} <b>[EXPIRED]</b>{{end}}{{if .Hints}}
<b>Hints</b>: {{range .Hints}}
- {{range .}}{{.ShortChannelId | channelLink}}: {{.PubKey | nodeAliasLink}}{{end}}{{end}}{{end}}
<b>Payee</b>: {{.Payee | nodeLink}} (<u>{{.Payee | nodeAlias}}</u>)

{{if .Sats}}Bezahle die oben beschriebene Rechnung?{{if .IsDiscord}}
Reagier mit einem :zap: um das zu bestätigen.{{end}}
{{else}}<b>Reply with the desired amount to confirm.</b>  Um zu bestätigen, antworte mit dem gewünschten Betrag.
{{end}}
    `,
	FAILEDDECODE: "Dekodieren der Rechnung gescheitert: {{.Err}}",
	BALANCEMSG: `🏛
<b>Full Balance</b>: {{printf "%.15g" .Sats}} sat ({{dollar .Sats}})
<b>Usable Balance</b>: {{printf "%.15g" .Usable}} sat ({{dollar .Usable}})
<b>Total received</b>: {{printf "%.15g" .Received}} sat
<b>Total sent</b>: {{printf "%.15g" .Sent}} sat
<b>Total fees paid</b>: {{printf "%.15g" .Fees}} sat

#balance
/transactions
    `,
	TAGGEDBALANCEMSG: `
<b>Total of</b> <code>received - spent</code> <b>on internal and third-party</b> /apps<b>:</b>

{{range .Balances}}<code>{{.Tag}}</code>: <i>{{printf "%.15g" .Balance}} sat</i>  ({{dollar .Balance}})
{{else}}
<i>No tagged transactions made yet.</i>
{{end}}
#balance
    `,
	FAILEDUSER: "Bei Analyse des Empfängernamens gescheitert.",
	LOTTERYMSG: `
Eine neue Lotterierunde ist gestartet!
Eintrittsgeld/Teilnahmegebühr: {{.EntrySats}} sat
Gesamtzahl an Teilnehmern: {{.Participants}}
Preis: {{.Prize}}
Registrierte Teilnehmer: {{.Registered}}
    `,
	INVALIDPARTNUMBER: "Ungültige Anzahl an Teilnehmern: {{.Number}}",
	USERSENTTOUSER:    "💛 {{menuItem .Sats .RawSats true }} ({{dollar .Sats}}) gesendet an {{.User}}{{if .ReceiverHasNoChat}} ({{.User}} konnte nicht informiert werden, weil dieser keinen Chat mit dem bot gestartet hat){{end}}.",
	USERSENTYOUSATS:   "💛 {{.User}} Nutzer hat dir gesendet {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}){{if .BotOp}} on a {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:  "💛 Jemand hat dir  {{menuItem .Sats .RawSats false}} ({{dollar .Sats}} gesendet).",
	FAILEDSEND:        "Senden fehlgeschlagen: ",
	QRCODEFAIL:        "QR Code konnte nicht erfolgreich gelesen werden: {{.Err}}",
	SAVERECEIVERFAIL:  "Speichern des Empfängers gescheitet. Das ist wahrscheinlich ein bug.",
	MISSINGRECEIVER:   "Fehlender Empfänger!",
	GIVERCANTJOIN:     "Der Geber kann niemals teilnehmen",
	CANTJOINTWICE:     "Du kannst nicht zweimal teilnehmen!",
	CANTREVEALOWN:     "Kann versteckte Nachrichten nicht aufdecken!",
	CANTCANCEL:        "Du verfügst nicht über die Rechte, das zu stornieren.",
	FAILEDINVOICE:     "Rechnungserstellung gescheitert: {{.Err}}",
	STOPNOTIFY:        "Benachrichtigungen gestoppt.",
	START: `
⚡️ @lntxbot, a <b>Bitcoin</b> Lightning Wallet in Deiner Telegram Anwendung.

🕹️  <b>Grundbefehle</b>
<b>&lt;invoice&gt;</b> - Füge einfach eine Rechnung oder eine LNURL ein, um es zu dekodieren oder zu bezahlen.
<b>/balance</b> - Zeigt dein Guthaben.
<b>/tip &lt;amount;&gt;</b> - Sende dies als Antwort auf eine andere Nachricht, um ein Trinkgeld zu geben.
<b>/invoice &lt;amount&gt; &lt;description&gt;</b> - Generiert eine Lightning Rechnung: <code>/invoice 400 'split coffee'</code>.
<b>/send &lt;amount;&gt; &lt;user&gt;</b> - Sende einige Satoshi an einen anderen Nutzer: <code>/send 100 @fiatjaf</code>

🍎 <b>Andere Dinge, die Du tun kannst</b>
- Verwende <b>/send</b> ,um Geld an irgendeine Adresse zu senden <a href="https://lightningaddress.com">Lightning Address</a>.
- Erhalte Geld auf {{ .YourName }}@lntxbot.com oder auf https://lntxbot.com/@{{ .YourName }}.
- Erstelle Berechnungen wie bspw. <code>4*usd</code> oder <code>eur*rand()</code> wann immer du einen Betrag in Satoshi spezifizieren möchtest.
- Benutze <b>/withdraw lnurl &lt;amount&gt;</b> um einen LNURL-Einlöse Gutschein zu generieren.

🎮 <b>Lustige oder nützliche Befehle</b>
<b>/sats4ads</b> Werde bezahlt um Spam Nachrichten zu erhalten, du kontrollierst wie viele -- oder schicke an jeden Anzeigen. Hohe Umsetzungsrate! 
<b>/giveaway</b> and <b>/giveflip</b> - Verschenke Geld in Gruppen!
<b>/hide</b> - Verstecke eine Nachricht, Personen müssen bezahlen, um sie lesen zu können. Mehrere Möglichkeiten der Enthüllung: öffentlich, privat, crowdfunded. Zahlreiche Medien unterstützt.
<b>/coinflip &lt;amount&gt; &lt;number_of_participants&gt;</b> - Erstellt einen Münzwurf, bei dem jeder mitmachen kann <i>(costs 10sat fee)</i>.

🐟 <b>Inline Commands</b> - <i>Können in jedem Chat verwendet werden, selbst dann, wenn der Bot nicht vorhanden ist.</i>
<code>@lntxbot give &lt;amount&gt;</code> - Kreiert in einem privaten Chat einen Button, um dem anderen Geld zu geben.
<code>@lntxbot coinflip/giveflip/giveaway</code> - Das Gleiche wie die slash-command Version, kann aber in Gruppen verwendet werden, die keinen @lntxbot installiert hat.
<code>@lntxbot invoice &lt;amount&gt;</code> - Erstellt eine Rechnung und sendet diese an einen Chat.

🏖  <b>Fortgeschrittene Befehle</b>
<b>/bluewallet</b> - Verbinde BlueWallet oder Zeus mit deinem @lntxbot Account.
<b>/transactions</b> - Listet alle deine Transaktionen auf, mit Seitennummerierung.
<b>/help &lt;command;&gt;</b> - Zeigt detaillierte Information für einen spezifischen Befehl an.
<b>/paynow &lt;invoice&gt;</b> -  bezahlt eine Rechnung, ohne vorher zu fragen.
<b>/send --anonymous &lt;amount&gt; &lt;user&gt;</b> - Der Empfänger weiß nicht, wer ihm/ihr Sats gesendet hat.

🏛  <b>Gruppenverwaltung</b>
<b>/toggle ticket &lt;amount&gt;</b> - Lege einen Eintrittspreis in Satoshi zum Betreten der Gruppe fest. Gut gegen Spammer! Das Geld geht an den Gruppeninhaber.
<b>/toggle renamable &lt;amount&gt;</b> - Erlaubt Personen deine Gruppe umzubennen und Du wirst dafür bezahlt. 
<b>/toggle expensive &lt;amount&gt; &lt;regex pattern&gt;</b> - Berechne Personen etwas, wenn sie die falschen Wörter in deiner Gruppe verwenden (oder lassen es frei, damit jede Nachricht kostenpflichtig ist).

---

Es gibt noch weitere Befehle, aber deren Erlernen bleibt zu Übungszwecken dem Nutzer überlassen.

Viel Erfolg! 🍽️
    `,
	WRONGCOMMAND:    "Konnte den Befehl nicht verstehen. /help",
	RETRACTQUESTION: "Nicht beanspruchtes Trinkgeld zurück ziehen?",
	RECHECKPENDING:  "Offene Zahlung erneut prüfen?",

	TXNOTFOUND: "Konnte Transaktion nicht finden {{.HashFirstChars}}.",
	TXINFO: `{{.Txn.Icon}} <code>{{.Txn.Status}}</code> {{.Txn.PeerActionDescription}} on {{.Txn.Time | time}} {{if .Txn.IsUnclaimed}}[💤 UNCLAIMED]{{end}}
<i>{{.Txn.Description}}</i>{{if .Txn.Tag.Valid}} #{{.Txn.Tag.String}}{{end}}{{if not .Txn.TelegramPeer.Valid}}
{{if .Txn.Payee.Valid}}<b>Payee</b>: {{.Txn.Payee.String | nodeLink}} (<u>{{.Txn.Payee.String | nodeAlias}}</u>){{end}}
<b>Hash</b>: <code>{{.Txn.Hash}}</code>{{end}}{{if .Txn.Preimage.String}}
<b>Preimage</b>: <code>{{.Txn.Preimage.String}}</code>{{end}}
<b>Amount</b>: <i>{{.Txn.Amount | printf "%.15g"}} sat</i> ({{dollar .Txn.Amount}})
{{if not (eq .Txn.Status "RECEIVED")}}<b>Fee paid</b>: <i>{{printf "%.15g" .Txn.Fees}} sat</i>{{end}}
{{.LogInfo}}
    `,
	TXLIST: `<b>{{if .Offset}}Transaktion von {{.From}} an {{.To}}{{else}}Latest {{.Limit}} transactions{{end}}</b>
{{range .Transactions}}<code>{{.StatusSmall}}</code> <code>{{.Amount | paddedSatoshis}}</code> {{.Icon}} {{.PeerActionDescription}}{{if not .TelegramPeer.Valid}}<i>{{.Description}}</i>{{end}} <i>{{.Time | timeSmall}}</i> /tx_{{.HashReduced}}
{{else}}
<i>Bisher keine Transaktion vorgenommen.</i>
{{end}}
    `,
	TXLOG: `<b>Routes tried</b>{{if .PaymentHash}} for <code>{{.PaymentHash}}</code>{{end}}:
{{range $t, $try := .Tries}}{{if $try.Success}}✅{{else}}❌{{end}} {{range $h, $hop := $try.Route}}➠{{.Channel | channelLink}}{{end}}{{with $try.Error}}{{if $try.Route}}
{{else}} {{end}}<i>{{. | makeLinks}}</i>
{{end}}{{end}}
    `,
}
