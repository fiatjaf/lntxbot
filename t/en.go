package t

var EN = map[Key]string{
	NO:         "No",
	YES:        "Yes",
	CANCEL:     "Cancel",
	CANCELED:   "Canceled.",
	COMPLETED:  "Completed!",
	CONFIRM:    "Confirm",
	FAILURE:    "Failure.",
	PROCESSING: "Processing...",
	WITHDRAW:   "Withdraw?",
	ERROR:      "Error{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:   "Checking...",
	TXCANCELED: "Transaction canceled.",
	UNEXPECTED: "Unexpected error: please report.",

	CALLBACKWINNER:        "Winner: {{.Winner}}",
	CALLBACKERROR:         "{{.BotOp}} error{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED:       "{{.BotOp}} expired.",
	CALLBACKATTEMPT:       "Attempting payment.",
	CALLBACKSENDING:       "Sending payment.",
	CALLBACKBUTTONEXPIRED: "The payment confirmation button has expired.",

	INLINEINVOICERESULT:  "Payment request for {{.Sats}} sat.",
	INLINEGIVEAWAYRESULT: "Give {{.Sats}} away",
	INLINEGIVEFLIPRESULT: "Give away {{.Sats}} sat to one out of {{.MaxPlayers}} participants",
	INLINECOINFLIPRESULT: "Lottery with entry fee of {{.Sats}} sat for {{.MaxPlayers}} participants",
	INLINEHIDDENRESULT:   "Message {{.HiddenId}}: {{.Content}}",

	LNURLINVALID: "Invalid lnurl: {{.Err}}",
	LNURLFAIL:    "Failed to fulfill lnurl withdraw: {{.Err}}",

	USERALLOWED:       "Invoice paid. {{.User}} allowed.",
	SPAMFILTERMESSAGE: "Hello, {{.User}}. You have 15min to pay the following invoice for {{.Sats}} sat if you want to stay in this group:",

	PAYMENTFAILED: "Payment failed. /log{{.ShortHash}}",
	PAIDMESSAGE: `Paid with <b>{{.Sats}} sat</b> (+ {{.Fee}} fee). 

<b>Hash:</b> {{.Hash}}
{{if .Preimage}}<b>Proof:</b> {{.Preimage}}{{end}}

/tx{{.ShortHash}}`,
	DBERROR:             "Database error: failed to mark the transaction as not pending.",
	INSUFFICIENTBALANCE: "Insufficient balance for {{.Purpose}}. Needs {{.Sats}}.0f sat more.",
	TOOSMALLPAYMENT:     "That's too small, please start your {{.Purpose}} with at least 40 sat.",

	PAYMENTRECEIVED:      "Payment received: {{.Sats}}. /tx{{.Hash}}.",
	FAILEDTOSAVERECEIVED: "Payment received, but failed to save on database. Please report this issue: <code>{{.Label}}</code>, hash: <code>{{.Hash}}</code>",

	SPAMMYMSG:    "{{if .Spammy}}This group is now spammy.{{else}}Not spamming anymore.{{end}}",
	TICKETMSG:    "New entrants will have to pay an invoice of {{.Sat}} sat (make sure you've set @{{.BotName}} as administrator for this to work).",
	FREEJOIN:     "This group is now free to join.",
	ASKTOCONFIRM: "Pay the invoice described above?",

	HELPINTRO: `
<pre>{{.Help}}</pre>
For more information on each command please type <code>/help &lt;command&gt;</code>.
    `,
	HELPSIMILAR: "/{{.Method}} command not found. Do you mean /{{index .Similar 0}}?{{if gt (len .Similar) 1}} Or maybe /{{index .Similar 1}}?{{if gt (len .Similar) 2}} Perhaps {{.}}?{{end}}{{end}}",
	HELPMETHOD: `
<pre>/{{.MainName}} {{.Argstr}}</pre>
{{.Desc}}
{{if .Examples}}
<b>Examples</b>
{{.Examples}}{{end}}
{{if .HasInline}}
<b>Inline query</b>
Can also be called as an <a href="https://core.telegram.org/bots/inline">inline query</a> from group or personal chats where the bot isn't added. The syntax is similar, but simplified: <code>@{{.ServiceId}} {{.InlineExample}}</code> then wait for a "search result" to appear.{{end}}
{{if .Aliases}}
<b>Aliases:</b> <code>{{.Aliases}}</code>{{end}}
    `,

	// the "any" is here only for illustrative purposes. if you call this with 'any' it will
	// actually be assigned to the <satoshis> variable, and that's how the code handles it.
	RECEIVEHELPARGS: "(lnurl <lnurl> | (<satoshis> | any) [<description>...] [--preimage=<preimage>])",
	RECEIVEHELPDESC: "Generates a BOLT11 invoice with given satoshi value. Amounts will be added to your bot balance. If you don't provide the amount it will be an open-ended invoice that can be paid with any amount.",
	RECEIVEHELPEXAMPLE: `
<code>/receive 320 for something</code>
Generates an invoice for 320 sat with the description "for something"

<code>/invoice any</code>
Generates an invoice with undefined amount.
    `,

	PAYHELPARGS: "(lnurl [<satoshis>] | [now] [<invoice>])",
	PAYHELPDESC: "Decodes a BOLT11 invoice and asks if you want to pay it (unless /paynow). This is the same as just pasting or forwarding an invoice directly in the chat. Taking a picture of QR code containing an invoice works just as well (if the picture is clear).",
	PAYHELPEXAMPLE: `
<code>/pay lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code>
Pays this invoice for 100 sat.

<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> 
Pays this invoice without asking for confirmation.

<code>/withdraw 3000</code> 
Generates an lnurl and QR code for withdrawing 3000 satoshis from a <a href="https://lightning-wallet.com">compatible wallet</a>.

<code>/pay</code> 
When sent as a reply to another message containing an invoice (for example, in a group), asks privately if you want to pay it.
    `,

	SENDHELPARGS: "[anonymously] <satoshis> [<receiver>...] [--anonymous]",
	SENDHELPDESC: "Sends satoshis to other Telegram users. The receiver is notified on his chat with the bot. If the receiver has never talked to the bot or have blocked it he can't be notified, however. In that case you can cancel the transaction afterwards in the /transactions view.",
	SENDHELPEXAMPLE: `
<code>/send 500 @username</code>
Sends 500 satoshis to Telegram user @username.

<code>/tip 100</code>
When sent as a reply to a message in a group where the bot is added, this will send 100 satoshis to the author of the message.

<code>/send anonymously 1000 @someone</code>
Telegram user @someone will see just: "Someone has sent you 1000 satoshis".
    `,

	BALANCEHELPDESC: "Shows your current balance in satoshis, plus the sum of everything you've received and sent within the bot and the total amount of fees paid.",

	GIVEAWAYHELPARGS: "<satoshis>",
	GIVEAWAYHELPDESC: "Creates a button in a group chat. The first person to click the button gets the satoshis.",
	GIVEAWAYHELPEXAMPLE: `
<code>/giveaway 1000</code>
Once someone clicks the 'Claim' button 1000 satoshis will be transferred from you to them.
    `,
	GIVEAWAYSATSGIVENPUBLIC: "{{.Sats}} sat given from {{.From}} to {{.To}}.{{if .ClaimerHasNoChat}} To manage your funds, start a conversation with @{{.BotName}}{{end}}.",
	CLAIMFAILED:             "Failed to claim {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:           "Claim",
	GIVEAWAYMSG:             "{{.User}} is giving {{.Sats}} sat away!",

	COINFLIPHELPARGS: "<satoshis> [<num_participants>]",
	COINFLIPHELPDESC: "Starts a fair lottery with the given number of participants. Everybody pay the same amount as the entry fee. The winner gets it all. Funds are only moved from participants accounts when the lottery is actualized.",
	COINFLIPHELPEXAMPLE: `
<code>/coinflip 100 5</code>
5 participants needed, winner will get 500 satoshis (including its own 100, so it's 400 net satoshis).
    `,
	COINFLIPWINNERMSG:      "You're the winner of a coinflip for a prize of {{.TotalSats}} sat. The losers were: {{.Senders}}.",
	COINFLIPGIVERMSG:       "You've lost {{.IndividualSats}} in a coinflip. The winner was {{.Receiver}}.",
	COINFLIPAD:             "Pay {{.Sats}} and get a chance to win {{.Prize}}! {{.SpotsLeft}} out of {{.MaxPlayers}} spots left!",
	COINFLIPJOIN:           "Join lottery!",
	CALLBACKCOINFLIPWINNER: "Coinflip winner: {{.Winner}}",
	COINFLIPOVERQUOTA:      "You're joining in too many coinflips! That can't be healthy!",
	COINFLIPRATELIMIT:      "Please wait a while before creating a new coinflip.",

	GIVEFLIPHELPARGS: "<satoshis> [<num_participants>]",
	GIVEFLIPHELPDESC: "Starts a giveaway, but instead of giving to the first person who clicks, the amount is raffled between first x clickers.",
	GIVEFLIPHELPEXAMPLE: `
<code>/giveflip 100 5</code>
5 participants needed, winner will get 500 satoshis from the command issuer.
    `,
	GIVEFLIPMSG:       "{{.User}} is giving {{.Sats}} sat away to a lucky person out of {{.Participants}}!",
	GIVEFLIPAD:        "{{.Sats}} being given away. Join and get a chance to win! {{.SpotsLeft}} out of {{.MaxPlayers}} spots left!",
	GIVEFLIPJOIN:      "Try to win!",
	GIVEFLIPWINNERMSG: "{{.Sender}} sent {{.Sats}} to {{.Receiver}}. These didn't get anything: {{.Losers}}.{{if .ReceiverHasNoChat}} To manage your funds, start a conversation with @{{.BotName}}{{end}}.",

	FUNDRAISEHELPARGS: "<satoshis> <num_participants> <receiver>...",
	FUNDRAISEHELPDESC: "Starts a crowdfunding event with a predefined number of participants and contribution amount. If the given number of participants contribute, it will be actualized. Otherwise it will be canceled in some hours.",
	FUNDRAISEHELPEXAMPLE: `
<code>/fundraise 10000 8 @user</code>
Telegram @user will get 80000 satoshis after 8 people contribute.
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

	BLUEWALLETHELPARGS: "[refresh]",
	BLUEWALLETHELPDESC: "Returns your credentials for importing your bot wallet on BlueWallet. You can use the same account from both places interchangeably.",
	BLUEWALLETHELPEXAMPLE: `
<code>/bluewallet</code>
Prints a string like "lndhub://&lt;login&gt;:&lt;password&gt;@&lt;url&gt;" which must be copied and pasted on BlueWallet's import screen.

<code>/bluewallet refresh</code>
Erases your previous password and prints a new string. You'll have to reimport the credentials on BlueWallet after this step. Only do it if your previous credentials were compromised.
    `,
	BLUEWALLETPASSWORDUPDATEERROR: "Error updating password. Please report this issue: {{.Err}}",
	BLUEWALLETCREDENTIALS:         "<code>{{.Credentials}}</code>",

	HIDEHELPARGS: "<satoshis> <message>...",
	HIDEHELPDESC: "Hides a message so it can be unlocked later with a payment. The special character \"~\" is used to split the message into a preview and the actual message (\"click here to see a secret! ~ this is the secret.\")",
	HIDEHELPEXAMPLE: `
<code>/hide 500 top secret message here</code>
Hides "top secret message" and returns an id for it. Later one will be able to make a reveal prompt for it using either /reveal &lt;hidden_message_id&gt; or by using the inline query "reveal" in a group.

<code>/hide 2500 only the brave will be able to see this message ~ congratulations, you are very brave!</code>
In this case instead of the default preview message potential revealers will see the custom teaser written before the "~".
    `,
	REVEALHELPARGS: "<hidden_message_id>",
	REVEALHELPDESC: "Reveals a message that was previously hidden. The author of the hidden message is never disclosed. Once a message is hidden it is available to be revealed globally, but only by those who know its hidden id.",
	REVEALHELPEXAMPLE: `
<code>/reveal 5c0b2rh4x</code>
Creates a prompt to reveal the hidden message 5c0b2rh4x, if it exists.
    `,
	HIDDENREVEALBUTTON:   "Pay {{.Sats}} sat to reveal the full message",
	HIDDENDEFAULTPREVIEW: "A message is hidden here. {{.Sats}} sat needed to unlock.",
	HIDDENWITHID:         "Message hidden with id <code>{{.HiddenId}}</code>.",
	HIDDENSOURCE:         "Hidden message <code>{{.Id}}</code> revealed by {{.Revealer}}. You've got {{.Sats}} sat.",
	HIDDENREVEAL:         "{{.Sats}} sat paid to reveal the message <code>{{.Id}}</code>.",
	HIDDENSTOREFAIL:      "Failed to store hidden content. Please report: {{.Err}}",
	HIDDENMSGFAIL:        "Failed to reveal: {{.Err}}",
	HIDDENMSGNOTFOUND:    "Hidden message not found.",

	APPHELPARGS: "(microbet [bet | bets | balance | withdraw] | bitflash [orders | status | rate | <satoshis> <address>] | satellite [transmissions | queue | bump <satoshis> <transmission_id> | delete <transmission_id> | <satoshis> <message>...] | golightning [<satoshis>] | poker [deposit <satoshis> | balance | withdraw | status | url | play | (available|watch|wait) <minutes>])",
	APPHELPDESC: "Interacts with external apps from within the bot and using your balance.",
	APPHELPEXAMPLE: `
<code>/app bitflash 1000000 3NRnMC5gVug7Mb4R3QHtKUcp27MAKAPbbJ</code>
Buys an onchain transaction to the given address using bitflash.club's shared fee feature. Will ask for confirmation.
<code>/app microbet bet</code>
Displays a list of currently opened bets from microbet.fun as buttons you can click to place back or lay bets.
<code>/app microbet bets</code>
Lists all your open bets. Your microbet.fun session will be tied to your Telegram user.
<code>/app satellite 26 hello from the satellite! vote trump!</code>
Queues a transmission from the Blockstream Satellite with a bid of 26 satoshis.
<code>/app golightning 1000000</code>
Creates an order to transfer 0.01000000 BTC from an on-chain address to your bot balance.
    `,

	BITFLASHCONFIRM:      `<b>[bitflash]</b> Do you confirm you want to queue a Bitflash transaction that will send <b>{{.BTCAmount}} BTC</b> to <code>{{.Address}}</code>? You will pay <b>{{printf "%.0f" .Sats}}</b>.`,
	BITFLASHTXQUEUED:     "Transaction queued!",
	BITFLASHFAILEDTOSAVE: "Failed to save Bitflash order. Please report: {{.Err}}",
	BITFLASHLIST: `
<b>[bitflash]</b> Your past orders
{{range .Orders}}🧱 <code>{{.Amount}}</code> to <code>{{.Address}}</code> <i>{{.Status}}</i>
{{else}}
<i>~ no orders were ever made. ~</i>
{{end}}
    `,
	BITFLASHHELP: `
<a href="https://bitflash.club/">Bitflash</a> is a service that does cheap onchain transactions from Lightning payments. It does it cheaply because it aggregates many Lightning transactions and then dispatches them to the chain after a certain threshold is reached.

<b>Commands:</b>

<code>/app bitflash &lt;satoshi_amount&gt; &lt;bitcoin_address&gt;</code> to queue a transaction.
<code>/app bitflash orders</code> lists your previous transactions.
    `,

	MICROBETBETHEADER:           "<b>[Microbet]</b> Bet on one of these predictions:",
	MICROBETINVALIDRESPONSE:     "microbet.fun returned an invalid response.",
	MICROBETPAIDBUTNOTCONFIRMED: "Paid, but bet not confirmed. Huge Microbet bug?",
	MICROBETPLACING:             "Placing bet on <b>{{.Bet.Description}}</b>.",
	MICROBETPLACED:              "Bet placed!",
	MICROBETFAILEDTOPAY:         "Failed to pay bet invoice.",
	MICROBETLIST: `
<b>[Microbet]</b> Your bets
{{range .Bets}}<code>{{.Description}}</code> <code>{{.Amount}}</code> {{if gt .UserBack 0}}{{.BackIcon}}{{.UserBack}}/{{.Backers}} × {{.Layers}}{{else}}{{.LayIcon}} {{.UserLay}}/{{.Layers}} × {{.Backers}}{{end}} ~<i>{{if .Canceled}}canceled{{else if .Closed}}{{if gt .WonAmount 0}}won {{.WonAmount}}{{else}}lost{{end}}{{else}}open{{end}}</i>
{{else}}
<i>~ no bets were ever made. ~</i>
{{end}}
    `,
	MICROBETBALANCEERROR: "Error fetching Microbet balance: {{.Err}}",
	MICROBETBALANCE:      "<b>[Microbet]</b> balance: <i>{{.Balance}} sat</i>",
	MICROBETHELP: `
<a href="https://microbet.fun/">Microbet</a> is a simple service that allows people to bet against each other on sports games results. The bet price is fixed and the odds are calculated considering the amount of back versus lay bets. There's a 1% fee on all withdraws.

<b>Commands:</b>

<code>/app microbet bet</code> to list all open bets and then place yours.
<code>/app microbet bets</code> to see all your past bets.
<code>/app microbet balance</code> to view your balance.
<code>/app microbet withdraw</code> to withdraw all your balance.
    `,

	SATELLITEFAILEDTOSTORE:     "Failed to store satellite order data. Please report: {{.Err}}",
	SATELLITEFAILEDTOGET:       "Failed to get stored satellite data: {{.Err}}",
	SATELLITEPAID:              "Transmission <code>{{.UUID}}</code> paid!",
	SATELLITEFAILEDTOPAY:       "Failed to pay for transmission.",
	SATELLITEBUMPERROR:         "Error bumping transmission: {{.Err}}",
	SATELLITEFAILEDTODELETE:    "Failed to delete satellite order data. Please report: {{.Err}}",
	SATELLITEDELETEERROR:       "Error deleting transmission: {{.Err}}",
	SATELLITEDELETED:           "Transmission deleted.",
	SATELLITETRANSMISSIONERROR: "Error making transmission: {{.Err}}",
	SATELLITEQUEUEERROR:        "Error fetching the queue: {{.Err}}",
	SATELLITEQUEUE: `
<b>[Satellite]</b> Queued transmissions
{{range .Orders}}{{.}}
{{else}}
<i>Queue is empty, everything was already transmitted.</i>
{{end}}
    `,
	SATELLITELIST: `
<b>[Satellite]</b> Your transmissions
{{range .Orders}}{{.}}
{{else}}
<i>No transmissions made yet.</i>
{{end}}
    `,
	SATELLITEHELP: `
The <a href="https://blockstream.com/satellite/">Blockstream Satellite</a> is a service that broadcasts Bitcoin blocks and other transmissions to the entire planet. You can transmit any message you want and pay with some satoshis.

<b>Commands:</b>

<code>/app satellite &lt;bid_satoshis&gt; &lt;message...&gt;</code> to queue a transmission.
<code>/app satellite transmissions</code> lists your transmissions.
<code>/app satellite queue</code> lists the next queued transmissions.
<code>/app satellite bump &lt;bid_increase_satoshis&gt; &lt;message_id&gt;</code> to increaase the bid for a transmission.
<code>/app satellite delete &lt;message_id&gt;</code> to delete a transmission.
    `,

	GOLIGHTNINGFAIL:   "<b>[GoLightning]</b> Failed to create order: {{.Err}}",
	GOLIGHTNINGFINISH: "<b>[GoLightning]</b> Finish your order by sending <code>{{.Order.Price}} BTC</code> to <code>{{.Order.Address}}</code>.",
	GOLIGHTNINGHELP: `
<a href="https://golightning.club/">GoLightning.club</a> is the cheapest way to get your on-chain funds to Lightning, at just 99 satoshi per order. First you specify how much you want to receive, then you send money plus fees to the provided BTC address. Done.

<b>Commands:</b>

<code>/app golightning &lt;satoshis&gt;</code> create an order for that number of satoshis.
    `,

	POKERDEPOSITFAIL:  "<b>[Poker]</b> Failed to deposit: {{.Err}}",
	POKERWITHDRAWFAIL: "<b>[Poker]</b> Failed to withdraw: {{.Err}}",
	POKERBALANCEERROR: "<b>[Poker]</b> Error fetching balance: {{.Err}}",
	POKERSECRETURL:    `<a href="{{.URL}}">Your personal secret Poker URL is here, never share it with anyone.</a>`,
	POKERBALANCE:      "<b>[Poker]</b> Balance: {{.Balance}}",
	POKERSTATUS: `
<b>[Poker]</b>
Players online: {{.Players}}
Active Tables: {{.Tables}}
Satoshis in play: {{.Chips}}
    `,
	POKERNOTIFY: `
<b>[Poker]</b> There are {{.Playing}} people playing {{if ne .Waiting 0}}and {{.Waiting}} waiting to play {{end}}poker right now{{if ne .Sats 0}} with a total of {{.Sats}} in play{{end}}!

/app_poker_status to double-check!
/app_poker_play to play here!
/app_poker_url to play in a browser window!
    `,
	POKERSUBSCRIBED: "You are available to play poker for the next {{.Minutes}} minutes.",
	POKERHELP: `
<a href="https://lightning-poker.com/">Lightning Poker</a> is the first and simplest multiplayer live No-Limit Texas Hold'em Poker game played directly with satoshis. Just join a table and start staking sats.

By playing from an account tied to your bot balance you can just sit on a table and your poker balance will be automatically refilled from your bot account, with minimal friction.

<b>Commands:</b>

<code>/app poker deposit &lt;satoshis&gt;</code> puts money in your poker bag.
<code>/app poker balance</code> shows how much you have there.
<code>/app poker withdraw</code> brings all the money back to the bot balance.
<code>/app poker status</code> tells you how active are the poker tables right now.
<code>/app poker url</code> displays the your <b>secret</b> game URL which you can open from any browser and gives access to your bot balance.
<code>/app poker play</code> displays the game widget.
<code>/app poker available &lt;minutes&gt;</code> will put you in a subscribed state on the game for the given time and notify other subscribed people you are waiting to play.
    `,

	TOGGLEHELPARGS: "(ticket [<price>]|spammy)",
	TOGGLEHELPDESC: "Toggles bot features in groups on/off. In supergroups it only be run by group admins.",
	TOGGLEHELPEXAMPLE: `
<code>/toggle ticket 10</code>
New group entrants will be prompted to pay 10 satoshis in 30 minutes or be kicked. Useful as an antispam measure.

<code>/toggle ticket</code>
Stop charging new entrants a fee.

<code>/toggle spammy</code>
'spammy' mode is off by default. When turned on, tip notifications will be sent in the group instead of only privately.
    `,

	HELPHELPARGS: "[<command>]",
	HELPHELPDESC: "Shows full help or help about specific command.",

	STOPHELPDESC: "The bot stops showing you notifications.",

	CONFIRMINVOICE: `
{{.Sats}} sat ({{.USD}})
<i>{{.Desc}}</i>
<b>Hash</b>: {{.Hash}}
<b>Node</b>: {{.Node}} ({{.Alias}})
    `,
	FAILEDDECODE: "Failed to decode invoice: {{.Err}}",
	NOINVOICE:    "Invoice not provided.",
	BALANCEMSG: `
<b>Balance</b>: {{printf "%.3f" .Sats}} sat ({{.USD}})
<b>Total received</b>: {{printf "%.3f" .Received}} sat
<b>Total sent</b>: {{printf "%.3f" .Sent}} sat
<b>Total fees paid</b>: {{printf "%.3f" .Fees}} sat
    `,
	// {{if ne .CoinflipBalance 0}}<b>Coinflip balance</b>: {{.CoinflipBalance}} sat ({{.CoinflipWins}} won, {{.CoinflipLoses}} lost)
	// {{end}}
	//     `,
	FAILEDUSER: "Failed to parse receiver name.",
	LOTTERYMSG: `
A lottery round is starting!
Entry fee: {{.EntrySats}} sat
Total participants: {{.Participants}}
Prize: {{.Prize}}
Registered: {{.Registered}}
    `,
	INVALIDPARTNUMBER:  "Invalid number of participants: {{.Number}}",
	INVALIDAMOUNT:      "Invalid amount: {{.Amount}}",
	USERSENTTOUSER:     "{{.Sats}} sat sent to {{.User}}{{if .ReceiverHasNoChat}} (couldn't notify {{.User}} as they haven't started a conversation with the bot){{end}}",
	USERSENTYOUSATS:    "{{.User}} has sent you {{.Sats}} sat{{if .BotOp}} on a {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:   "Someone has sent you {{.Sats}} sat.",
	FAILEDSEND:         "Failed to send: ",
	QRCODEFAIL:         "QR code reading unsuccessful: {{.Err}}",
	SAVERECEIVERFAIL:   "Failed to save receiver. This is probably a bug.",
	CANTSENDNORECEIVER: "Can't send {{.Sats}}. Missing receiver!",
	GIVERCANTJOIN:      "Giver can't join!",
	CANTJOINTWICE:      "Can't join twice!",
	CANTCANCEL:         "You don't have the powers to cancel this.",
	FAILEDINVOICE:      "Failed to generate invoice: {{.Err}}",
	ZEROAMOUNTINVOICE:  "Invoices with undefined amounts are not supported because they are not safe.",
	INVALIDAMT:         "Invalid amount: {{.Amount}}",
	STOPNOTIFY:         "Notifications stopped.",
	WELCOME:            "Your account is created.",
	WRONGCOMMAND:       "Could not understand the command. /help",
	RETRACTQUESTION:    "Retract unclaimed tip?",
	RECHECKPENDING:     "Recheck pending payment?",
	TXNOTFOUND:         "Couldn't find transaction {{.HashFirstChars}}.",
	TXINFO: `<code>{{.Txn.Status}}</code> {{.Txn.PeerActionDescription}} on {{.Txn.TimeFormat}} {{if .Txn.IsUnclaimed}}(💤y unclaimed){{end}}
<i>{{.Txn.Description}}</i>{{if not .Txn.TelegramPeer.Valid}}
{{if .Txn.Payee.Valid}}<b>Payee</b>: {{.Txn.PayeeLink}} ({{.Txn.PayeeAlias}}){{end}}
<b>Hash</b>: {{.Txn.Hash}}{{end}}{{if .Txn.Preimage.String}}
<b>Preimage</b>: {{.Txn.Preimage.String}}{{end}}
<b>Amount</b>: {{.Txn.Amount}} sat
{{if not (eq .Txn.Status "RECEIVED")}}<b>Fee paid</b>: {{.Txn.FeeSatoshis}}{{end}}
{{.LogInfo}}
    `,
	TXLIST: `<b>{{if .Offset}}Transactions from {{.From}} to {{.To}}{{else}}Latest {{.Limit}} transactions{{end}}</b>
{{range .Transactions}}<code>{{.StatusSmall}}</code> <code>{{.PaddedSatoshis}}</code> {{.Icon}} {{.PeerActionDescription}}{{if not .TelegramPeer.Valid}}<i>{{.Description}}</i>{{end}} <i>{{.TimeFormatSmall}}</i> /tx{{.HashReduced}}
{{else}}
<i>No transactions made yet.</i>
{{end}}
    `,
}
