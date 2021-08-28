package t

var EN = map[Key]string{
	NO:         "No",
	YES:        "Yes",
	CANCEL:     "Cancel",
	CANCELED:   "Canceled.",
	COMPLETED:  "Completed!",
	CONFIRM:    "Confirm",
	PAYAMOUNT:  `Pay {{.Sats | printf "%.15g"}}`,
	FAILURE:    "Failure.",
	PROCESSING: "Processing...",
	WITHDRAW:   "Withdraw?",
	ERROR:      "{{if .App}}#{{.App | lower}} {{end}}Error{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:   "Checking...",
	TXPENDING:  "Payment still in flight, please try checking again later.",
	TXCANCELED: "Transaction canceled.",
	UNEXPECTED: "Unexpected error: please report.",

	CALLBACKWINNER:  "Winner: {{.Winner}}",
	CALLBACKERROR:   "{{.BotOp}} error{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED: "{{.BotOp}} expired.",
	CALLBACKATTEMPT: "Attempting payment. /tx_{{.Hash}}",
	CALLBACKSENDING: "Sending payment.",

	INLINEINVOICERESULT:  "Payment request for {{.Sats}} sat.",
	INLINEGIVEAWAYRESULT: "Give {{.Sats}} sat {{if .Receiver}}to @{{.Receiver}}{{else}}away{{end}}",
	INLINEGIVEFLIPRESULT: "Give away {{.Sats}} sat to one out of {{.MaxPlayers}} participants",
	INLINECOINFLIPRESULT: "Lottery with entry fee of {{.Sats}} sat for {{.MaxPlayers}} participants",
	INLINEHIDDENRESULT:   "{{.HiddenId}} ({{if gt .Message.Crowdfund 1}}crowd:{{.Message.Crowdfund}}{{else if gt .Message.Times 0}}priv:{{.Message.Times}}{{else if .Message.Public}}pub{{else}}priv{{end}}): {{.Message.Content}}",

	LNURLUNSUPPORTED: "That kind of lnurl is not supported here.",
	LNURLERROR:       `<b>{{.Host}}</b> lnurl error: {{.Reason}}`,
	LNURLAUTHSUCCESS: `
lnurl-auth success!

<b>Domain</b>: <i>{{.Host}}</i>
<b>Public Key</b>: <i>{{.PublicKey}}</i>
`,
	LNURLPAYPROMPT: `<code>{{.Domain}}</code> expects {{if .FixedAmount}}<i>{{.FixedAmount | printf "%.15g"}} sat</i>{{else}}a value between <i>{{.Min | printf "%.15g"}}</i> and <i>{{.Max | printf "%.15g"}} sat</i>{{end}} for:

{{if .Text}}<code>{{.Text | html}}</code>{{end}}

{{if not .FixedAmount}}<b>Reply with the amount to confirm.</b>{{end}}
    `,
	LNURLPAYPROMPTCOMMENT: `<code>{{.Domain}}</code> expects some text:`,
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
	LNURLPAYCOMMENT:           "Along with /tx_{{.HashFirstChars}} you got a message: \n\n<i>{{.Text}}</i>",
	LNURLBALANCECHECKCANCELED: "Automatic balance checks from {{.Service}} are cancelled.",

	USERALLOWED:       "Invoice paid. {{.User}} allowed.",
	SPAMFILTERMESSAGE: "Hello, {{.User}}. You have 15min to pay the following invoice for {{.Sats}} sat if you want to stay in this group:",

	RENAMABLEMSG:      "Anyone can rename this group as long as they pay {{.Sat}} sat (make sure you've set @lntxbot as administrator for this to work).",
	RENAMEPROMPT:      "Pay <b>{{.Sats}} sat</b> to rename this group to <i>{{.Name}}</i>?",
	GROUPNOTRENAMABLE: "This group is not renamable!",

	INTERNALPAYMENTUNEXPECTED: "Something odd has happened. If this is an internal invoice it will fail. Maybe the invoice has expired or something else we don't know. If it is an external invoice ignore this warning.",
	PAYMENTFAILED:             "Payment failed. /log_{{.ShortHash}}",
	PAIDMESSAGE: `Paid with <i>{{printf "%.15g" .Sats}} sat</i> ({{dollar .Sats}}) (+ <i>{{.Fee}}</i> fee). 

<b>Hash:</b> <code>{{.Hash}}</code>{{if .Preimage}}
<b>Proof:</b> <code>{{.Preimage}}</code>{{end}}

/tx_{{.ShortHash}} #tx`,
	OVERQUOTA:           "You're over your {{.App}} weekly quota.",
	RATELIMIT:           "This action is rate-limited. Please wait 30 minutes.",
	DBERROR:             "Database error: failed to mark the transaction as not pending.",
	INSUFFICIENTBALANCE: `Insufficient balance for {{.Purpose}}. Needs {{.Sats | printf "%.15g"}} sat more.`,

	PAYMENTRECEIVED:      "Payment received: {{.Sats}} sat ({{dollar .Sats}}). /tx_{{.Hash}} #tx",
	FAILEDTOSAVERECEIVED: "Payment received, but failed to save on database. Please report this issue: <code>{{.Hash}}</code>",

	SPAMMYMSG:             "{{if .Spammy}}This group is now spammy.{{else}}Not spamming anymore.{{end}}",
	COINFLIPSENABLEDMSG:   "Coinflips are {{if .Enabled}}enabled{{else}}disabled{{end}} in this group.",
	LANGUAGEMSG:           "This chat language is set to <code>{{.Language}}</code>.",
	TICKETMSG:             "New entrants will have to pay an invoice of {{.Sat}} sat (make sure you've set @lntxbot as administrator for this to work).",
	FREEJOIN:              "This group is now free to join.",
	EXPENSIVEMSG:          "Every message in this group{{with .Pattern}} containing the pattern <code>{{.}}</code>{{end}} will cost {{.Price}} sat.",
	EXPENSIVENOTIFICATION: "The message {{.Link}} just {{if .Sender}}cost{{else}}earned{{end}} you {{.Price}} sat.",
	FREETALK:              "Messages are free again",

	APPBALANCE: `#{{.App | lower}} Balance: <i>{{printf "%.15g" .Balance}} sat</i>`,

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

	HIDEHELP: `Hides a message so it can be unlocked later with a payment. The special character "~" is used to split the message into a preview and the actual message ("click here to see a secret! ~ this is the secret.")

<code>/hide 500 'top secret message here'</code> hides "top secret message" and returns an id for it. Later one will be able to make a reveal prompt for it using either /reveal &lt;hidden_message_id&gt;
<code>/hide 2500 'only the rich will be able to see this message' ~ 'congratulations, you are very rich!'</code>: in this case instead of the default preview message potential revealers will see the custom teaser written before the "~".

Anyone can reveal any message (after paying) by typing <code>/reveal &lt;hidden_message_id&gt;</code> in their private chat with the bot, but the id is only known to the message creator unless it shares it.

The basic way to share the message, however, is to click the "share" button and use the inline query in a group or chat. That will post the message preview to the chat along with a button people can click to pay and reveal.  You can control if the message will be revealed in-place for the entire group to see or privately just to the payer using the <code>--public</code> flag. By default it's private.

You can also control how many people will be allowed to reveal it privately using the <code>--revealers</code> argument or how many will be required to pay before it is revealed publicly with the <code>--crowdfund</code> argument.

<code>/hide 100 'three people have paid for this message to be revealed' --crowdfund 3</code>: the message will be revealed publicly once 3 people pay 100 satoshis.
<code>/hide 321 'only 3 people can see this message' ~ "you're one among 3 privileged" --revealers 3</code>: the message will be revealed privately to the first 3 people who click.
    `,
	REVEALHELP: `Reveals a message that was previously hidden. The author of the hidden message is never disclosed. Once a message is hidden it is available to be revealed globally, but only by those who know its hidden id.

A reveal prompt can also be created in a group or chat by clicking the "share" button after you hide the message, then the standard message reveal rules apply, see /help_hide for more info.

<code>/reveal 5c0b2rh4x</code> creates a prompt to reveal the hidden message 5c0b2rh4x, if it exists.
    `,
	HIDDENREVEALBUTTON:   `{{.Sats}} to reveal {{if .Public}}in-place{{else}}privately{{end}}. {{if gt .Crowdfund 1}}Crowdfunding: {{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Revealers allowed: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "A message is hidden here. {{.Sats}} sat needed to unlock.",
	HIDDENWITHID:         "Message hidden with id <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Will be revealed publicly once {{.Message.Crowdfund}} people pay {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Will be revealed privately to the first {{.Message.Times}} payers{{else if .Message.Public}}Will be revealed publicly once one person pays {{.Message.Satoshis}}{{else}}Will be revealed privately to any payer{{end}}.",
	HIDDENSOURCEMSG:      "Hidden message <code>{{.Id}}</code> revealed by {{.Revealers}}. You got {{.Sats}} sat.",
	HIDDENREVEALMSG:      "{{.Sats}} sat paid to reveal the message <code>{{.Id}}</code>.",
	HIDDENMSGNOTFOUND:    "Hidden message not found.",
	HIDDENSHAREBTN:       "Share in another chat",

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
	BALANCEMSG: `
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
	INVALIDPARTNUMBER:  "Invalid number of participants: {{.Number}}",
	USERSENTTOUSER:     "{{menuItem .Sats .RawSats true }} ({{dollar .Sats}}) sent to {{.User}}{{if .ReceiverHasNoChat}} (couldn't notify {{.User}} as they haven't started a conversation with the bot){{end}}.",
	USERSENTYOUSATS:    "{{.User}} has sent you {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}){{if .BotOp}} on a {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:   "Someone has sent you {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}).",
	FAILEDSEND:         "Failed to send: ",
	QRCODEFAIL:         "QR code reading unsuccessful: {{.Err}}",
	SAVERECEIVERFAIL:   "Failed to save receiver. This is probably a bug.",
	CANTSENDNORECEIVER: "Can't send {{.Sats}}. Missing receiver!",
	GIVERCANTJOIN:      "Giver can't join!",
	CANTJOINTWICE:      "Can't join twice!",
	CANTREVEALOWN:      "Can't reveal your own hidden message!",
	CANTCANCEL:         "You don't have the powers to cancel this.",
	FAILEDINVOICE:      "Failed to generate invoice: {{.Err}}",
	STOPNOTIFY:         "Notifications stopped.",
	START: `
⚡️ @lntxbot, a <b>Bitcoin</b> Lightning wallet on your Telegram.

🕹️  <b>Basic Commands</b>
<b>&lt;invoice&gt;</b> - Just paste an invoice or an LNURL to decode or pay it.
<b>/balance</b> - Shows your balance.
<b>/tip &ltamount;&gt;</b> - Send this in reply to another message in a group to tip.
<b>/invoice &lt;amount&gt; &lt;description&gt;</b> - Generates a Lightning invoice: <code>/invoice 400 'split coffee'</code>.
<b>/send &ltamount;&gt; &lt;user&gt;</b> - Sends some satoshis to another user: <code>/send 100 @fiatjaf</code>

🫒 <b>Other things you can do</b>
- Use <b>/send</b> to send money to any <a href="https://lightningaddress.com">Lightning Address</a>.
- Use <b>/withdraw lnurl &lt;amount&gt;</b> to create an LNURL-withdraw voucher.
- Receive money at yourname@lntxbot.com or at https://lntxbot.com/@yourname.

🎮 <b>Fun or useful commands</b>
<b>/sats4ads</b> Get paid to receive spam messages, you control how much -- or send ads to everybody. Big conversion rates! 
<b>/giveaway</b> and <b>/giveflip</b> - Give money away in groups!
<b>/hide</b> - Hide a message, people will have to pay to see it. Multiple ways of revealing: public, private, crowdfunded.
<b>/coinflip &lt;amount&gt; &lt;number_of_participants&gt;</b> - Creates a lottery anyone can join <i>(costs 10sat fee)</i>.

🪕 <b>Inline Commands</b> - <i>Can be used in any chat, even if the bot is not present</i>
<b>@lntxbot give &lt;amount&gt;</b> - Creates a button in a private chat to give money to the other side.
<b>@lntxbot coinflip/giveflip/giveaway &lt;amount&gt; &lt;number_of_participants&gt;</b> - Same as the slash-command version, but can be used in groups without @lntxbot.
<b>@lntxbot invoice &lt;amount&gt;</b> - Makes an invoice and sends it to chat.

🫕 <b>Advanced Commands</b>
<b>/bluewallet</b> - Connect BlueWallet or Zeus to your @lntxbot account.
<b>/transactions</b> - Lists all your transactions, paginated.
<b>/help &ltcommand;&gt;</b> - Shows detailed help for a specific command.
<b>/paynow &lt;invoice&gt;</b> - Pays an invoice without asking.
<b>/sendnonymously &lt;amount&gt; &lt;user&gt;</b> - Like /send, but anonymous.

🫓 <b>Group Administration</b>
<b>/toggle ticket &lt;amount&gt;</b> - Put a price in satoshis for joining your group. Great antispam! Money goes to group owner.
<b>/toggle renamable &lt;amount&gt;</b> - Allows people to use /rename to rename your group and you get paid.
<b>/toggle expensive &lt;amount&gt; &lt;regex pattern&gt;</b> - Charge people for saying the wrong words in your group (or left blank to charge for all messages).
---

There are other commands, but learning them is left as an exercise to the user.

Good luck! 🍽️
    `,
	WRONGCOMMAND:    "Could not understand the command. /help",
	RETRACTQUESTION: "Retract unclaimed tip?",
	RECHECKPENDING:  "Recheck pending payment?",

	TXNOTFOUND: "Couldn't find transaction {{.HashFirstChars}}.",
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
	TXLOG: `<b>Routes tried</b>:
{{range $t, $try := .Tries}}{{if $try.Success}}✅{{else}}❌{{end}} {{range $h, $hop := $try.Route}}➠<code>{{msatToSat .Msatoshi | printf "%.15g"}}</code>➠{{.Channel | channelLink}}{{end}}{{with $try.Error}}{{if $try.Route}}
{{else}} {{end}}<i>{{. | makeLinks}}</i>
{{end}}{{end}}
    `,
}
