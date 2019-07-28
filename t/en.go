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

	CALLBACKWINNER:  "Winner: {{.Winner}}",
	CALLBACKERROR:   "{{.BotOp}} error{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED: "{{.BotOp}} expired.",
	CALLBACKATTEMPT: "Attempting payment.",
	CALLBACKSENDING: "Sending payment.",

	INLINEINVOICERESULT:  "Payment request for {{.Sats}} sat.",
	INLINEGIVEAWAYRESULT: "Give {{.Sats}} away",
	INLINEGIVEFLIPRESULT: "Give away {{.Sats}} sat to one out of {{.MaxPlayers}} participants",
	INLINECOINFLIPRESULT: "Lottery with entry fee of {{.Sats}} sat for {{.MaxPlayers}} participants",
	INLINEHIDDENRESULT:   "{{.HiddenId}} ({{if gt .Message.Crowdfund 1}}crowd:{{.Message.Crowdfund}}{{else if gt .Message.Times 0}}priv:{{.Message.Times}}{{else if .Message.Public}}pub{{else}}priv{{end}}): {{.Message.Content}}",

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
	HELPSIMILAR: "/{{.Method}} command not found. Do you mean /{{index .Similar 0}}?{{if gt (len .Similar) 1}} Or maybe /{{index .Similar 1}}?{{if gt (len .Similar) 2}} Perhaps {{index .Similar 2}}?{{end}}{{end}}",
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
	RECEIVEHELP: `Generates a BOLT11 invoice with given satoshi value. Amounts will be added to your bot balance. If you don't provide the amount it will be an open-ended invoice that can be paid with any amount.",

<code>/receive_320_for_something</code> generates an invoice for 320 sat with the description "for something"
<code>/receive 100 for hidden data --preimage="0000000000000000000000000000000000000000000000000000000000000000"</code> generates an invoice with the given preimage (beware, you might lose money, only use if you know what you're doing).
    `,

	PAYHELP: `Decodes a BOLT11 invoice and asks if you want to pay it (unless /paynow). This is the same as just pasting or forwarding an invoice directly in the chat. Taking a picture of QR code containing an invoice works just as well (if the picture is clear).

Just pasting <code>lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> decodes and prompts to pay the given invoice.  
<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> pays the given invoice invoice without asking for confirmation.
/withdraw_lnurl_3000 generates an lnurl and QR code for withdrawing 3000 satoshis from a <a href="https://lightning-wallet.com">compatible wallet</a> without asking for confirmation.
/withdraw_lnurl generates an lnurl and QR code for withdrawing any amount, but will ask for confirmation in the bot chat.
<code>/pay</code>, when sent as a reply to another message containing an invoice (for example, in a group), asks privately if you want to pay it.
    `,

	SENDHELP: `Sends satoshis to other Telegram users. The receiver is notified on his chat with the bot. If the receiver has never talked to the bot or have blocked it he can't be notified, however. In that case you can cancel the transaction afterwards in the /transactions view.

<code>/tip 100</code>, when sent as a reply to a message in a group where the bot is added, sends 100 satoshis to the author of the message.
<code>/send 500 @username</code> sends 500 satoshis to Telegram user @username.
<code>/send anonymously 1000 @someone</code> same as above, but telegram user @someone will see just: "Someone has sent you 1000 satoshis".
    `,

	BALANCEHELP: "Shows your current balance in satoshis, plus the sum of everything you've received and sent within the bot and the total amount of fees paid.",

	GIVEAWAYHELP: `Creates a button in a group chat. The first person to click the button gets the satoshis.

/giveaway_1000: once someone clicks the 'Claim' button 1000 satoshis will be transferred from you to them.
    `,
	GIVEAWAYSATSGIVENPUBLIC: "{{.Sats}} sat given from {{.From}} to {{.To}}.{{if .ClaimerHasNoChat}} To manage your funds, start a conversation with @{{.BotName}}.{{end}}",
	CLAIMFAILED:             "Failed to claim {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:           "Claim",
	GIVEAWAYMSG:             "{{.User}} is giving {{.Sats}} sat away!",

	COINFLIPHELP: `Starts a fair lottery with the given number of participants. Everybody pay the same amount as the entry fee. The winner gets it all. Funds are only moved from participants accounts when the lottery is actualized.

/coinflip_100_5: 5 participants needed, winner will get 500 satoshis (including its own 100, so it's 400 net satoshis).
    `,
	COINFLIPWINNERMSG:      "You're the winner of a coinflip for a prize of {{.TotalSats}} sat. The losers were: {{.Senders}}.",
	COINFLIPGIVERMSG:       "You've lost {{.IndividualSats}} in a coinflip. The winner was {{.Receiver}}.",
	COINFLIPAD:             "Pay {{.Sats}} and get a chance to win {{.Prize}}! {{.SpotsLeft}} out of {{.MaxPlayers}} spots left!",
	COINFLIPJOIN:           "Join lottery!",
	CALLBACKCOINFLIPWINNER: "Coinflip winner: {{.Winner}}",
	COINFLIPOVERQUOTA:      "You're over your coinflip daily quota.",
	COINFLIPRATELIMIT:      "Please wait 30 minutes before creating a new coinflip.",

	GIVEFLIPHELP: `Starts a giveaway, but instead of giving to the first person who clicks, the amount is raffled between first x clickers.

/giveflip_100_5: 5 participants needed, winner will get 500 satoshis from the command issuer.
    `,
	GIVEFLIPMSG:       "{{.User}} is giving {{.Sats}} sat away to a lucky person out of {{.Participants}}!",
	GIVEFLIPAD:        "{{.Sats}} being given away. Join and get a chance to win! {{.SpotsLeft}} out of {{.MaxPlayers}} spots left!",
	GIVEFLIPJOIN:      "Try to win!",
	GIVEFLIPWINNERMSG: "{{.Sender}} sent {{.Sats}} to {{.Receiver}}. These didn't get anything: {{.Losers}}.{{if .ReceiverHasNoChat}} To manage your funds, start a conversation with @{{.BotName}}.{{end}}",

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

	BLUEWALLETHELP: `Returns your credentials for importing your bot wallet on BlueWallet. You can use the same account from both places interchangeably.

/bluewallet prints a string like "lndhub://&lt;login&gt;:&lt;password&gt;@&lt;url&gt;" which must be copied and pasted on BlueWallet's import screen.
/bluewallet_refresh erases your previous password and prints a new string. You'll have to reimport the credentials on BlueWallet after this step. Only do it if your previous credentials were compromised.
    `,
	BLUEWALLETPASSWORDUPDATEERROR: "Error updating password. Please report this issue: {{.Err}}",
	BLUEWALLETCREDENTIALS:         "<code>{{.Credentials}}</code>",

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
	HIDDENREVEALBUTTON:   `{{.Sats}} to reveal {{if .Public}} in-place{{else }} privately{{end}}. {{if gt .Crowdfund 1}}Crowdfunding: {{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Revealers allowed: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "A message is hidden here. {{.Sats}} sat needed to unlock.",
	HIDDENWITHID:         "Message hidden with id <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Will be revealed publicly once {{.Message.Crowdfund}} people pay {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Will be revealed privately to the first {{.Message.Times}} payers{{else if .Message.Public}}Will be revealed publicly once one person pays {{.Message.Satoshis}}{{else}}Will be revealed privately to any payer{{end}}.",
	HIDDENSOURCEMSG:      "Hidden message <code>{{.Id}}</code> revealed by {{.Revealers}}. You've got {{.Sats}} sat.",
	HIDDENREVEALMSG:      "{{.Sats}} sat paid to reveal the message <code>{{.Id}}</code>.",
	HIDDENSTOREFAIL:      "Failed to store hidden content. Please report: {{.Err}}",
	HIDDENMSGNOTFOUND:    "Hidden message not found.",
	HIDDENSHAREBTN:       "Share in another chat",

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

/app_bitflash_100000_3NRnMC5gVug7Mb4R3QHtKUcp27MAKAPbbJ buys an onchain transaction to the given address using bitflash.club's shared fee feature. Will ask for confirmation.
/app_bitflash_orders</code> lists your previous transactions.
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

/app_microbet_bet displays all open bet markets so you can yours.
/app_microbet_bets shows your bet history.
/app_microbet_balance displays your balance.
/app_microbet_withdraw withdraws all your balance.
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

<code>/app satellite 13 'hello from the satellite! vote trump!'</code> queues that transmission to the satellite with a bid of 13 satoshis.
/app_satellite_transmissions lists your transmissions.
    `,

	GOLIGHTNINGFAIL:   "<b>[GoLightning]</b> Failed to create order: {{.Err}}",
	GOLIGHTNINGFINISH: "<b>[GoLightning]</b> Finish your order by sending <code>{{.Order.Price}} BTC</code> to <code>{{.Order.Address}}</code>.",
	GOLIGHTNINGHELP: `
<a href="https://golightning.club/">GoLightning.club</a> is the cheapest way to get your on-chain funds to Lightning, at just 99 satoshi per order. First you specify how much you want to receive, then you send money plus fees to the provided BTC address. Done.

/app_golightning_1000000 creates an order to transfer 0.01000000 BTC from an on-chain address to your bot balance.
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
	POKERHELP: `<a href="https://lightning-poker.com/">Lightning Poker</a> is the first and simplest multiplayer live No-Limit Texas Hold'em Poker game played directly with satoshis. Just join a table and start staking sats.

By playing from an account tied to your bot balance you can just sit on a table and your poker balance will be automatically refilled from your bot account, with minimal friction.

/app_poker_deposit_10000 puts 10000 satoshis in your poker bag.
/app_poker_balance shows how much you have there.
/app_poker_withdraw brings all the money back to the bot balance.
/app_poker_status tells you how active are the poker tables right now.
/app_poker_url displays your <b>secret</b> game URL which you can open from any browser and gives access to your bot balance.
/app_poker_play displays the game widget.
/app_poker_watch_120 will put you in a subscribed state on the game for 2 hours and notify other subscribed people you are waiting to play. You'll be notified whenever there were people playing.
    `,

	TOGGLEHELP: `Toggles bot features in groups on/off. In supergroups it can only be run by admins.

<code>/toggle ticket 10</code> starts charging a fee for all new entrants. Useful as an antispam measure. The money goes to the group owner.
<code>/toggle ticket</code> stops charging new entrants a fee. 
<code>/toggle spammy</code>: 'spammy' mode is off by default. When turned on, tip notifications will be sent in the group instead of only privately.
    `,

	HELPHELP: "Shows full help or help about specific command.",

	STOPHELP: "The bot stops showing you notifications.",

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
