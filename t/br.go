package t

var PT = map[Key]string{
	NO:          "Nao",
	YES:         "Sim",
	CANCEL:      "Cancelar",
	CANCELED:    "Cancelado.",
	COMPLETED:   "Finalizado!",
	CONFIRM:     "Confirmar",
	PAYAMOUNT:   `Pague {{.Sats | printf "%.15g"}}`,
	FAILURE:     "Falha.",
	PROCESSING:  "Processando...",
	WITHDRAW:    "Retirar?",
	ERROR:       "🔴 {{if .App}}#{{.App | lower}} {{end}}Erro{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:    "Verificando...",
	TXPENDING:   "Enviando pagamento, por favor, verifique novamente mais tarde.",
	TXCANCELED:  "Transação cancelada.",
  UNEXPECTED:  "Erro inesperado: por favor, informe.",
	MUSTBEADMIN: "Esse comando precisa ser executado por um admin.",
	MUSTBEGROUP: "Esse comando precisa ser utilizado em um grupo.",

	CALLBACKWINNER:  "Ganhador: {{.Winner}}",
	CALLBACKERROR:   "{{.BotOp}} erro{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED: "{{.BotOp}} expirado.",
	CALLBACKATTEMPT: "Tentando pagamento. /tx_{{.Hash}}",
	CALLBACKSENDING: "Enviando pagamento.",

	INLINEINVOICERESULT:  "Pagamento requisitado para {{.Sats}} sat.",
	INLINEGIVEAWAYRESULT: "Dê {{.Sats}} sat {{if .Receiver}}para @{{.Receiver}}{{else}}away{{end}}",
	INLINEGIVEFLIPRESULT: "Dê {{.Sats}} sat para um dos {{.MaxPlayers}} participantes",
	INLINECOINFLIPRESULT: "Sorteio com taxa de entrada de {{.Sats}} sat para {{.MaxPlayers}} participantes",
	INLINEHIDDENRESULT:   "{{.HiddenId}} ({{if gt .Message.Crowdfund 1}}crowd:{{.Message.Crowdfund}}{{else if gt .Message.Times 0}}priv:{{.Message.Times}}{{else if .Message.Public}}pub{{else}}priv{{end}}): {{.Message.Content}}",

	LNURLUNSUPPORTED: "Esse tipo de lnurl não é compatível aqui.",
	LNURLERROR:       `<b>{{.Host}}</b> erro lnurl: {{.Reason}}`,
	LNURLAUTHSUCCESS: `
lnurl-auth successo!

<b>Domnio</b>: <i>{{.Host}}</i>
<b>Chave pública</b>: <i>{{.PublicKey}}</i>
`,
	LNURLPAYPROMPT: `🟢 <code>{{.Domain}}</code> expects {{if .FixedAmount}}<i>{{.FixedAmount | printf "%.15g"}} sat</i>{{else}}a value between <i>{{.Min | printf "%.15g"}}</i> and <i>{{.Max | printf "%.15g"}} sat</i>{{end}} for:

<code>{{if .Long}}{{.Long | html}}{{else}}{{.Text | html}}{{end}}</code>{{if .WillSendPayerData}}

---

- Your name and/or auth keys will be sent to the payee.
- To prevent that, use <code>/lnurl --anonymous &lt;lnurl&gt;</code>.
{{end}}

{{if not .FixedAmount}}<b>Reply with the amount (in satoshis, between <i>{{.Min | printf "%.15g"}}</i> and <i>{{.Max | printf "%.15g"}}</i>) to confirm.</b>{{end}}
    `,
	LNURLPAYPROMPTCOMMENT: `📨 <code>{{.Domain}}</code> precisa de um comentário.

<b>To confirm the payment, reply with some text</b>`,
	LNURLPAYAMOUNTSNOTICE: `<code>{{.Domain}}</code> expected {{if .Exact}}{{.Min | printf "%.3f"}}{{else if .NoMax}}at least{{.Min | printf "%.0f"}}{{else}}between {{.Min | printf "%.0f"}} and {{.Max | printf "%.0f"}}{{end}} sat.`,
	LNURLPAYSUCCESS: `<code>{{.Domain}}</code> says:
{{.Text}}
{{if .DecipherError}}Failed to decipher ({{.DecipherError}}):
{{end}}{{if .Value}}<pre>{{.Value}}</pre>
{{end}}{{if .URL}}<a href="{{.URL}}">{{.URL}}</a>{{end}}
    `,
	LNURLPAYMETADATA: `#lnurlpay metadata:
<b>domain</b>: <i>{{.Domain}}</i>
<b>transaction</b>: /tx_{{.HashFirstChars}}
    `,
	LNURLBALANCECHECKCANCELED: "Automatic balance checks from {{.Service}} are cancelled.",

	TICKETSET:         "Novos participantes deverão pagar a fatura de {{.Sat}} sat (se certifique que @lntxbot é administrador do grupo para que funcione).",
	TICKETUSERALLOWED: "Fatura paga. {{.User}} liberado.",
	TICKETMESSAGE: `⚠️ {{.User}}, esse grupo requisita que você pague {{.Sats}} sat para poder entrar.

Você tem 15 minutos para pagar ou será retirado do grupo e permanecerá banido por um dia.
`,

	RENAMABLEMSG:      "Qualquer um pode renomear esse grupo, contanto que pague {{.Sat}} sat (se certifique que @lntxbot é administrador do grupo para que funcione).",
	RENAMEPROMPT:      "Pague <b>{{.Sats}} sat</b> para renomear esse grupo para <i>{{.Name}}</i>?",
	GROUPNOTRENAMABLE: "Esse grupo não pode ser renomeado!",

	INTERNALPAYMENTUNEXPECTED: "Something odd has happened. If this is an internal invoice it will fail. Maybe the invoice has expired or something else we don't know. If it is an external invoice ignore this warning.",
	PAYMENTFAILED:             "❌ Payment failed. /log_{{.ShortHash}}",
	PAIDMESSAGE: `✅ Paid with <i>{{printf "%.15g" .Sats}} sat</i> ({{dollar .Sats}}) (+ <i>{{.Fee}}</i> fee). 

<b>Hash:</b> <code>{{.Hash}}</code>{{if .Preimage}}
<b>Proof:</b> <code>{{.Preimage}}</code>{{end}}

/tx_{{.ShortHash}} ⚡️ #tx`,
	OVERQUOTA:           "You're over your {{.App}} weekly quota.",
	RATELIMIT:           "This action is rate-limited. Please wait 30 minutes.",
	DBERROR:             "Database error: failed to mark the transaction as not pending.",
	INSUFFICIENTBALANCE: `Insufficient balance for {{.Purpose}}. Needs {{.Sats | printf "%.15g"}} sat more.`,

	PAYMENTRECEIVED: `
      ⚡️ Payment received{{if .SenderName}} from <i>{{ .SenderName }}</i>{{end}}: {{.Sats}} sat ({{dollar .Sats}}). /tx_{{.Hash}}{{if .Message}} {{.Message | messageLink}}{{end}} #tx
      {{if .Comment}}
📨 <i>{{.Comment}}</i>
      {{end}}
    `,
	FAILEDTOSAVERECEIVED: "Payment received, but failed to save on database. Please report this issue: <code>{{.Hash}}</code>",

	SPAMMYMSG:             "{{if .Spammy}}This group is now spammy.{{else}}Not spamming anymore.{{end}}",
	COINFLIPSENABLEDMSG:   "Coinflips are {{if .Enabled}}enabled{{else}}disabled{{end}} in this group.",
	LANGUAGEMSG:           "O idioma dessa conversa está configurada para <code>{{.Language}}</code>.",
	FREEJOIN:              "Esse grupo está aberto para livre entrada.",
	EXPENSIVEMSG:          "Every message in this group{{with .Pattern}} containing the pattern <code>{{.}}</code>{{end}} will cost {{.Price}} sat.",
	EXPENSIVENOTIFICATION: "The message {{.Link}} just {{if .Sender}}cost{{else}}earned{{end}} you {{.Price}} sat.",
	FREETALK:              "Mensagens estão liberadas novamente",

	APPBALANCE: `#{{.App | lower}} Saldo: <i>{{printf "%.15g" .Balance}} sat</i>`,

	HELPINTRO: `
<pre>{{.Help}}</pre>
For more information on each command type <code>/help &lt;command&gt;</code>.
    `,
	HELPSIMILAR: "/{{.Method}} command not found. Do you mean /{{index .Similar 0}}?{{if gt (len .Similar) 1}} Or maybe /{{index .Similar 1}}?{{if gt (len .Similar) 2}} Perhaps /{{index .Similar 2}}?{{end}}{{end}}",
	HELPMETHOD: `
<pre>/{{.MainName}} {{.Argstr}}</pre>
{{.Help}}
{{if .HasInline}}
<b>Inline query</b>
Can also be called as an <a href="https://core.telegram.org/bots/inline">inline query</a> from group or personal chats where the bot isn't added. The syntax is similar, but simplified: <code>@{{.ServiceId}} {{.InlineExample}}</code> then wait for a "search result" to appear.{{end}}
{{if .Aliases}}
<b>Aliases:</b> <code>{{.Aliases}}</code>{{end}}
    `,

	// the "any" is here only for illustrative purposes. if you call this with 'any' it will
	// actually be assigned to the <satoshis> variable, and that's how the code handles it.
	RECEIVEHELP: `Generates a BOLT11 invoice with given satoshi value. Amounts will be added to your @lntxbot balance. If you don't provide the amount it will be an open-ended invoice that can be paid with any amount.",

<code>/receive_320_for_something</code> generates an invoice for 320 sat with the description "for something"
    `,

	PAYHELP: `Decodes a BOLT11 invoice and asks if you want to pay it (unless /paynow). This is the same as just pasting or forwarding an invoice directly in the chat. Taking a picture of QR code containing an invoice works just as well (if the picture is clear).

Just pasting <code>lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> decodes and prompts to pay the given invoice.  

<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> pays the given invoice invoice without asking for confirmation.

/withdraw_lnurl_3000 generates an <b>lnurl and QR code for withdrawing 3000</b> satoshis from a <a href="https://lightning-wallet.com">compatible wallet</a> without asking for confirmation.
    `,

	SENDHELP: `Sends satoshis to other Telegram users. The receiver is notified on his chat with @lntxbot. If the receiver has never talked to the bot or have blocked it he can't be notified, however. In that case you can cancel the transaction afterwards in the /transactions view.

<code>/tip 100</code>, when sent as a reply to a message in a group where the bot is added, sends 100 satoshis to the author of the message.
<code>/send 500 @username</code> sends 500 satoshis to Telegram user @username.
<code>/send anonymously 1000 @someone</code> same as above, but telegram user @someone will see just: "Someone has sent you 1000 satoshis".
    `,

	TRANSACTIONSHELP: `
Lists all your transactions with pagination controls. Each transaction has a link that can be clicked for more information.

/transactions lists all transactions, from the most recent.
<code>/transactions --in</code> lists only the incoming transactions.
<code>/transactions --out</code> lists only the outgoing transactions.
    `,

	BALANCEHELP: "Shows your current balance in satoshis, plus the sum of everything you've received and sent within the bot and the total amount of fees paid.",

	FINEHELP: "Prompts a user in a group to pay a fee. If they don't pay within 15 minutes they are kicked from the group and banned for a day.",
	FINEMESSAGE: `⚠️ {{.FinedUser}}, you were <b>fined</b> for <i>{{.Sats}} sat</i>{{if .Reason}} for <i>{{ .Reason }}</i>{{end}}.

You have 15 minutes to pay or you will be kicked.
    `,
	FINEFAILURE: "{{.User}} failed to pay the fine and was kicked and banned for one day.",
	FINESUCCESS: "{{.User}} has paid the fine.",

	GIVEAWAYHELP: `Creates a button in a group chat. The first person to click the button gets the satoshis.

/giveaway_1000: once someone clicks the 'Claim' button 1000 satoshis will be transferred from you to them.
    `,
	SATSGIVENPUBLIC: "{{.Sats}} sat given from {{.From}} to {{.To}}.{{if .ClaimerHasNoChat}} To manage your funds, start a conversation with @lntxbot.{{end}}",
	CLAIMFAILED:     "Failed to claim {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:   "Claim",
	GIVEAWAYMSG:     "{{.User}} is giving {{if .Away}}away{{else if .Receiver}}@{{.Receiver}}{{else}}you{{end}} {{.Sats}} sats!",

	COINFLIPHELP: `Starts a fair lottery with the given number of participants. Everybody pay the same amount as the entry fee. The winner gets it all. Funds are only moved from participants accounts when the lottery is actualized.

/coinflip_100_5: 5 participants needed, winner will get 500 satoshis (including its own 100, so it's 400 net satoshis).
    `,
	COINFLIPWINNERMSG:      "You're the winner of a coinflip for a prize of {{.TotalSats}} sat. The losers were: {{.Senders}}.",
	COINFLIPGIVERMSG:       "You've lost {{.IndividualSats}} in a coinflip. The winner was {{.Receiver}}.",
	COINFLIPAD:             "Pay {{.Sats}} and get a chance to win {{.Prize}}! {{.SpotsLeft}} out of {{.MaxPlayers}} spot{{s .SpotsLeft}} left!",
	COINFLIPJOIN:           "Join lottery!",
	CALLBACKCOINFLIPWINNER: "Coinflip winner: {{.Winner}}",

	GIVEFLIPHELP: `Starts a giveaway, but instead of giving to the first person who clicks, the amount is raffled between first x clickers.

/giveflip_100_5: 5 participants needed, winner will get 500 satoshis from the command issuer.
    `,
	GIVEFLIPMSG:       "{{.User}} is giving {{.Sats}} sat away to a lucky person out of {{.Participants}}!",
	GIVEFLIPAD:        "{{.Sats}} being given away. Join and get a chance to win! {{.SpotsLeft}} out of {{.MaxPlayers}} spot{{s .SpotsLeft}} left!",
	GIVEFLIPJOIN:      "Try to win!",
	GIVEFLIPWINNERMSG: "{{.Sender}} sent {{.Sats}} to {{.Receiver}}. These didn't get anything: {{.Losers}}.{{if .ReceiverHasNoChat}} To manage your funds, start a conversation with @lntxbot.{{end}}",

	FUNDRAISEHELP: `Starts a crowdfunding event with a predefined number of participants and contribution amount. If the given number of participants contribute, it will be actualized. Otherwise it will be canceled in some hours.

<code>/fundraise 10000 8 @user</code>: Telegram @user will get 80000 satoshis after 8 people contribute.
    `,
	FUNDRAISEAD: `
Fundraising {{.Fund}} to {{.ToUser}}!
Contributors needed for completion: {{.Participants}}
Each pays: {{.Sats}} sat
Have contributed: {{.Registered}}
    `,
	FUNDRAISEJOIN:        "Contribute!",
	FUNDRAISECOMPLETE:    "Fundraising for {{.Receiver}} completed!",
	FUNDRAISERECEIVERMSG: "You've received {{.TotalSats}} sat of a fundraise from {{.Senders}}s",
	FUNDRAISEGIVERMSG:    "You've given {{.IndividualSats}} in a fundraise to {{.Receiver}}.",

	LIGHTNINGATMHELP: `Returns the credentials in the format expected by @Z1isenough's <a href="https://docs.lightningatm.me">LightningATM</a>.

For specific documentation on how to setup it with @lntxbot visit <a href="https://docs.lightningatm.me/lightningatm-setup/wallet-setup/lntxbot">the lntxbot setup tutorial</a> (there's also <a href="https://docs.lightningatm.me/faq-and-common-problems/wallet-communication#talking-to-an-api-in-practice">a more detailed and technical background</a>).
  `,
	BLUEWALLETHELP: `Returns your credentials for importing your bot wallet on BlueWallet. You can use the same account from both places interchangeably.

/bluewallet prints a string like "lndhub://&lt;login&gt;:&lt;password&gt;@&lt;url&gt;" which must be copied and pasted on BlueWallet's import screen.
/bluewallet_refresh erases your previous password and prints a new string. You'll have to reimport the credentials on BlueWallet after this step. Only do it if your previous credentials were compromised.
    `,
	APIPASSWORDUPDATEERROR: "Error updating password. Please report: {{.Err}}",
	APICREDENTIALS: `
These are tokens for <i>Basic Auth</i>. The API is compatible with lndhub.io with some extra methods.

Full access: <code>{{.Full}}</code>
Invoice access: <code>{{.Invoice}}</code>
Read-only access: <code>{{.ReadOnly}}</code>
API Base URL: <code>{{.ServiceURL}}/</code>

/api_full, /api_invoice and /api_readonly will show these specific tokens along with QR codes.
/api_url will show a QR code for the API Base URL.

Keep these tokens secret. If they leak for some reason call /api_refresh to replace all.
    `,

	HIDEHELP: `Hides a message so it can be unlocked later with a payment.
<code>/hide 500 'teaser showed on prompt'</code>, send this in reply to any message, with video, audio, images or text, and it will be hidden behind a 500 satoshis paywall.

Modifiers:
  <code>--crowdfund &lt;number&gt;</code> enables public crowdfunding of hidden messages.
  <code>--private</code> reveals the hidden message privately to the payer instead of in the group.
  <code>--revealers &lt;number&gt;</code> only allows the first <code>&lt;number&gt;</code> participants to see the hidden the message, then the prompt expires.
    `,
	REVEALHELP: `Reveals a message that was previously hidden. The author of the hidden message is never disclosed. Once a message is hidden it is available to be revealed globally, but only by those who know its hidden id.

A reveal prompt can also be created in a group or chat by clicking the "share" button after you hide the message, then the standard message reveal rules apply, see /help_hide for more info.

<code>/reveal 5c0b2rh4x</code> creates a prompt to reveal the hidden message 5c0b2rh4x, if it exists.
    `,
	HIDDENREVEALBUTTON:   `{{.Sats}} sat to reveal {{if .Public}}in-place{{else}}privately{{end}}. {{if gt .Crowdfund 1}}{{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Left: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "A message is hidden here. {{.Sats}} sat needed to unlock.",
	HIDDENWITHID: `Message hidden with id <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Will be revealed publicly once {{.Message.Crowdfund}} people pay {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Will be revealed privately to the first {{.Message.Times}} payers{{else if .Message.Public}}Will be revealed publicly once one person pays {{.Message.Satoshis}}{{else}}Will be revealed privately to any payer{{end}}.

{{if .WithInstructions}}Call /reveal_{{.HiddenId}} on a group to share it there.{{end}}
    `,
	HIDDENSOURCEMSG:   "Hidden message <code>{{.Id}}</code> revealed by {{.Revealers}}. You got {{.Sats}} sat.",
	HIDDENREVEALMSG:   "{{.Sats}} sat paid to reveal the message <code>{{.Id}}</code>.",
	HIDDENMSGNOTFOUND: "Hidden message not found.",
	HIDDENSHAREBTN:    "Share in another chat",

	TOGGLEHELP: `Toggles bot features in groups on/off. In supergroups it can only be run by admins.

/toggle_ticket_10 starts charging a fee for all new entrants. Useful as an antispam measure. The money goes to the group owner.
/toggle_ticket stops charging new entrants a fee. 
/toggle_language_ru changes the chat language to Russian, /toggle_language displays the chat language, these also work in private chats.
/toggle_spammy toggles 'spammy' mode. 'spammy' mode is off by default. When turned on, tip notifications will be sent in the group instead of only privately.
    `,

	SATS4ADSHELP: `
Sats4ads is an ad marketplace on Telegram. Pay money to show ads to others, receive money for each ad you see.

Rates for each user are in msatoshi-per-character. The maximum rate is 1000 msat.
Each ad also includes a fixed fee of 1 sat.
Images and videos are priced as if they were 100 characters.
Links are priced an extra 300 characters as they have an annoying preview.

To broadcast an ad you must send a message to the bot that will be your ad contents, then reply to it using <code>/sats4ads broadcast ...</code> as described. You can use <code>--max-rate=500</code> and <code>--skip=0</code> to have better control over how your message is going to be broadcasted. These are the defaults.

/sats4ads_on_15 puts your account in ad-listening mode. Anyone will be able to publish messages to you for 15 msatoshi-per-character. You can adjust that price.
/sats4ads_off turns off your account so you won't get any more ads.
/sats4ads_rates shows a breakdown of how many nodes are at each price level. Useful to plan your ad budget early.
/sats4ads_rate shows your rate.
/sats4ads_preview in reply to a message shows a preview of how other users will see it. The satoshi amount shown in the preview message is not meaningful.
/sats4ads_broadcast_1000 broadcasts an ad. The last number is the maximum number of satoshis that will be spend. Cheaper ad-listeners will be preferred over more expensive ones. Must be called in a reply to another message, the contents of which will be used as the ad text.
    `,
	SATS4ADSTOGGLE:    `#sats4ads {{if .On}}Seeing ads and receiving {{printf "%.15g" .Sats}} sat per character.{{else}}You won't see any more ads.{{end}}`,
	SATS4ADSBROADCAST: `#sats4ads {{if .NSent}}Message broadcasted {{.NSent}} time{{s .NSent}} for a total cost of {{.Sats}} sat ({{dollar .Sats}}).{{else}}Couldn't find a peer to notify with the given parameters. /sats4ads_rates{{end}}`,
	SATS4ADSSTART:     `Message being broadcasted.`,
	SATS4ADSPRICETABLE: `#sats4ads Quantity of users <b>up to</b> each pricing tier.
{{range .Rates}}<code>{{.UpToRate}} msat</code>: <i>{{.NUsers}} user{{s .NUsers}}</i>
{{else}}
<i>No one is registered to see ads yet.</i>
{{end}}
Each ad costs the above prices <i>per character</i> + <code>1 sat</code> for each user.
    `,
	SATS4ADSADFOOTER: `[#sats4ads: {{printf "%.15g" .Sats}} sat]`,
	SATS4ADSVIEWED:   `Claim`,

	HELPHELP: "Shows full help or help about specific command.",

	STOPHELP: "The bot stops showing you notifications.",

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

{{if .Sats}}Pay the invoice described above?{{if .IsDiscord}}
React with a :zap: to confirm.{{end}}
{{else}}<b>Reply with the desired amount to confirm.</b>
{{end}}
    `,
	FAILEDDECODE: "Failed to decode invoice: {{.Err}}",
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
	FAILEDUSER: "Failed to parse receiver name.",
	LOTTERYMSG: `
A lottery round is starting!
Entry fee: {{.EntrySats}} sat
Total participants: {{.Participants}}
Prize: {{.Prize}}
Registered: {{.Registered}}
    `,
	INVALIDPARTNUMBER: "Número de participantes inválido: {{.Number}}",
	USERSENTTOUSER:    "💛 {{menuItem .Sats .RawSats true }} ({{dollar .Sats}}) enviado para {{.User}}{{if .ReceiverHasNoChat}} (não foi possível notificar {{.User}} por ele não ter iniciado uma conversa com o bot){{end}}.",
	USERSENTYOUSATS:   "💛 {{.User}} enviou para você {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}){{if .BotOp}} on a {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:  "💛 Alguém enviou para você {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}).",
	FAILEDSEND:        "Falha de envio: ",
	QRCODEFAIL:        "A leitura do QR coide falhou: {{.Err}}",
	SAVERECEIVERFAIL:  "Falha em salvar o destinatário. Isso provavelmente é um bug.",
	MISSINGRECEIVER:   "Missing receiver!",
	GIVERCANTJOIN:     "Giver can't join!",
	CANTJOINTWICE:     "Can't join twice!",
	CANTREVEALOWN:     "Não foi possível revelar sua mensagem escondida!",
	CANTCANCEL:        "Você não tem o poder de cancelar isso.",
	FAILEDINVOICE:     "Falhou em gerar fatura: {{.Err}}",
	STOPNOTIFY:        "As notificações foram suspensas.",
	START: `
⚡️ @lntxbot, uma carteira <b>Bitcoin</b> Lightning no seu Telegram.

🕹️  <b>Comandos básicos</b>
<b>&lt;invoice&gt;</b> - Apenas cole uma fatura ou um LNURL para decodificar ou pagar..
<b>/balance</b> - Mostre seu saldo.
<b>/tip &lt;amount;&gt;</b> -Envie isso em resposta a uma outra mensagem em um grupo para dar gorjeta.
<b>/invoice &lt;amount&gt; &lt;description&gt;</b> - Gere uma fatura Lightning: <code>/invoice 400 'dividir conta do café'</code>.
<b>/send &lt;amount;&gt; &lt;user&gt;</b> - Envie alguns satoshis para outro usuário: <code>/send 100 @messarelos</code>

🍎 <b>Outras coisas que você pode fazer</b>
- Use <b>/send</b> para enviar dinheiro para qualquer <a href="https://lightningaddress.com">Lightning Address</a>.
- Receba dinheiro em {{ .YourName }}@lntxbot.com ou em https://lntxbot.com/@{{ .YourName }}.
- Do calculations like <code>4*usd</code> or <code>eur*rand()</code> whenever you would specify an amount in satoshis.
- Use <b>/withdraw lnurl &lt;amount&gt;</b> para criar um voucher LNURL-withdraw.

🎮 <b>Diversão ou comandos úteis</b>
<b>/sats4ads</b> Seja pago para receber mensagens spam, você controla o quanto -- ou envie propaganda para todos. Taxas de conversão altas! 
<b>/giveaway</b> and <b>/giveflip</b> - Give money away in groups!
<b>/hide</b> - Esconda uma mensagem, pessoas pagarão para vê-las. Múltiplas formas de revelar: pública, privada, vaquinha. Suporte a multimídia.
<b>/coinflip &lt;amount&gt; &lt;number_of_participants&gt;</b> - Crie uma loteria que todos possam participar <i>(taxa de custo 10sat)</i>.

🐟 <b>Comandos Inline</b> - <i>Can be used in any chat, even if the bot is not present</i>
<code>@lntxbot give &lt;amount&gt;</code> - Creates a button in a private chat to give money to the other side.
<code>@lntxbot coinflip/giveflip/giveaway</code> - Same as the slash-command version, but can be used in groups without @lntxbot.
<code>@lntxbot invoice &lt;amount&gt;</code> - Makes an invoice and sends it to chat.

🏖  <b>Comandos avançados</b>
<b>/bluewallet</b> - Conecte a BlueWallet ou Zeus à sua conta @lntxbot.
<b>/transactions</b> - Liste todas as suas transações, paginadas.
<b>/help &lt;command;&gt;</b> - Mostra ajuda detalhada para um comando específico.
<b>/paynow &lt;invoice&gt;</b> - Pague uma fatura sem perguntas.
<b>/send --anonymous &lt;amount&gt; &lt;user&gt;</b> - O destinatário não saberá quem enviou sats para ele.

🏛  <b>Administração de grupo</b>
<b>/toggle ticket &lt;amount&gt;</b> - Ponha um preço em sats para entrar no seu grupo. Grande antispamm! O dinheiro vai para o dono do grupo.
<b>/toggle renamable &lt;amount&gt;</b> - Permite que as pessoas usem /rename para renomear seu grupo e você será pago.
<b>/toggle expensive &lt;amount&gt; &lt;regex pattern&gt;</b> - Cobre pessoas por dizerem coisas proibidas em seu grupo (ou deixe em branco para cobrar por todas as mensagens).
<b>/fine &lt;amount&gt;</b> - Faça pessoas pagarem ou elas serão chutadas para fora do grupo.

---

There are other commands, but learning them is left as an exercise to the user.

Good luck! 🍽️
    `,
	WRONGCOMMAND:    "Não foi possível entender o comando. /help",
	RETRACTQUESTION: "Retract unclaimed tip?",
	RECHECKPENDING:  "Verificar pagamento pendente novamente?",

	TXNOTFOUND: "Não foi possível encontrar a transação {{.HashFirstChars}}.",
	TXINFO: `{{.Txn.Icon}} <code>{{.Txn.Status}}</code> {{.Txn.PeerActionDescription}} on {{.Txn.Time | time}} {{if .Txn.IsUnclaimed}}[💤 UNCLAIMED]{{end}}
<i>{{.Txn.Description}}</i>{{if .Txn.Tag.Valid}} #{{.Txn.Tag.String}}{{end}}{{if not .Txn.TelegramPeer.Valid}}
{{if .Txn.Payee.Valid}}<b>Payee</b>: {{.Txn.Payee.String | nodeLink}} (<u>{{.Txn.Payee.String | nodeAlias}}</u>){{end}}
<b>Hash</b>: <code>{{.Txn.Hash}}</code>{{end}}{{if .Txn.Preimage.String}}
<b>Preimage</b>: <code>{{.Txn.Preimage.String}}</code>{{end}}
<b>Amount</b>: <i>{{.Txn.Amount | printf "%.15g"}} sat</i> ({{dollar .Txn.Amount}})
{{if not (eq .Txn.Status "RECEIVED")}}<b>Fee paid</b>: <i>{{printf "%.15g" .Txn.Fees}} sat</i>{{end}}
{{.LogInfo}}
    `,
	TXLIST: `<b>{{if .Offset}}Transactions from {{.From}} to {{.To}}{{else}}Latest {{.Limit}} transactions{{end}}</b>
{{range .Transactions}}<code>{{.StatusSmall}}</code> <code>{{.Amount | paddedSatoshis}}</code> {{.Icon}} {{.PeerActionDescription}}{{if not .TelegramPeer.Valid}}<i>{{.Description}}</i>{{end}} <i>{{.Time | timeSmall}}</i> /tx_{{.HashReduced}}
{{else}}
<i>No transactions made yet.</i>
{{end}}
    `,
	TXLOG: `<b>Routes tried</b>{{if .PaymentHash}} for <code>{{.PaymentHash}}</code>{{end}}:
{{range $t, $try := .Tries}}{{if $try.Success}}✅{{else}}❌{{end}} {{range $h, $hop := $try.Route}}➠{{.Channel | channelLink}}{{end}}{{with $try.Error}}{{if $try.Route}}
{{else}} {{end}}<i>{{. | makeLinks}}</i>
{{end}}{{end}}
    `,
}
