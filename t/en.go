package t

var EN = map[Key]string{
	NO:         "No",
	YES:        "Yes",
	CANCEL:     "Cancel",
	CANCELED:   "Canceled.",
	COMPLETED:  "Completed!",
	CONFIRM:    "Confirm",
	PAYAMOUNT:  "Pay {{.Sats}}",
	FAILURE:    "Failure.",
	PROCESSING: "Processing...",
	WITHDRAW:   "Withdraw?",
	ERROR:      "{{if .App}}<b>[{{.App}}]</b> {{end}}Error{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:   "Checking...",
	TXPENDING:  "Payment still in flight, please try checking again later.",
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

	LNURLUNSUPPORTED: "That kind of lnurl is not supported here.",
	LNURLAUTHSUCCESS: `
lnurl-auth success!

<b>domain</b>: <i>{{.Host}}</i>
<b>key</b>: <i>{{.PublicKey}}</i>
`,
	LNURLPAYPROMPT: `<code>{{.Domain}}</code> expects {{if .FixedAmount}}<i>{{.FixedAmount | printf "%.3f"}} sat</i>{{else}}a value between <i>{{.Min | printf "%.3f"}}</i> and <i>{{.Max | printf "%.3f"}} sat</i>{{end}} for the following:

{{if .Text}}<code>{{.Text | html}}</code>{{end}}

{{if not .FixedAmount}}<b>Reply with the amount to confirm.</b>{{end}}
    `,
	LNURLPAYSUCCESS: `<code>{{.Domain}}</code> says:

{{if .DecipherError}}Failed to decipher ({{.DecipherError}}):
{{end}}<pre>{{.Text}}</pre>
{{if .URL}}<a href="{{.URL}}">{{.URL}}</a>{{end}}
    `,
	LNURLPAYMETADATA: `lnurl-pay metadata:
<b>domain</b>: <i>{{.Domain}}</i>
<b>lnurl</b>: <i>{{.LNURL}}</i>
<b>transaction</b>: <i>{{.Hash}}</i> /tx{{.HashFirstChars}}
    `,

	USERALLOWED:       "Invoice paid. {{.User}} allowed.",
	SPAMFILTERMESSAGE: "Hello, {{.User}}. You have 15min to pay the following invoice for {{.Sats}} sat if you want to stay in this group:",

	PAYMENTFAILED: "Payment failed. /log{{.ShortHash}}",
	PAIDMESSAGE: `Paid with <b>{{printf "%.3f" .Sats}} ({{dollar .Sats}}) sat</b> (+ {{.Fee}} fee). 

<b>Hash:</b> {{.Hash}}{{if .Preimage}}
<b>Proof:</b> {{.Preimage}}{{end}}

/tx{{.ShortHash}}`,
	OVERQUOTA:           "You're over your {{.App}} daily quota.",
	RATELIMIT:           "This action is rate-limited. Please wait 30 minutes.",
	DBERROR:             "Database error: failed to mark the transaction as not pending.",
	INSUFFICIENTBALANCE: "Insufficient balance for {{.Purpose}}. Needs {{.Sats}}.0f sat more.",

	PAYMENTRECEIVED:      "Payment received: {{.Sats}} sat ({{dollar .Sats}}). /tx{{.Hash}}.",
	FAILEDTOSAVERECEIVED: "Payment received, but failed to save on database. Please report this issue: <code>{{.Label}}</code>, hash: <code>{{.Hash}}</code>",

	SPAMMYMSG:           "{{if .Spammy}}This group is now spammy.{{else}}Not spamming anymore.{{end}}",
	COINFLIPSENABLEDMSG: "Coinflips are {{if .Enabled}}enabled{{else}}disabled{{end}} in this group.",
	LANGUAGEMSG:         "This chat language is set to <code>{{.Language}}</code>.",
	TICKETMSG:           "New entrants will have to pay an invoice of {{.Sat}} sat (make sure you've set @{{.BotName}} as administrator for this to work).",
	FREEJOIN:            "This group is now free to join.",

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
	RECEIVEHELP: `Generates a BOLT11 invoice with given satoshi value. Amounts will be added to your @{{ .BotName }} balance. If you don't provide the amount it will be an open-ended invoice that can be paid with any amount.",

<code>/receive_320_for_something</code> generates an invoice for 320 sat with the description "for something"
<code>/receive 100 for hidden data --preimage="0000000000000000000000000000000000000000000000000000000000000000"</code> generates an invoice with the given preimage (beware, you might lose money, only use if you know what you're doing).
    `,

	PAYHELP: `Decodes a BOLT11 invoice and asks if you want to pay it (unless /paynow). This is the same as just pasting or forwarding an invoice directly in the chat. Taking a picture of QR code containing an invoice works just as well (if the picture is clear).

Just pasting <code>lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> decodes and prompts to pay the given invoice.  
<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> pays the given invoice invoice without asking for confirmation.
/withdraw_lnurl_3000 generates an lnurl and QR code for withdrawing 3000 satoshis from a <a href="https://lightning-wallet.com">compatible wallet</a> without asking for confirmation.
/withdraw_lnurl generates an lnurl and QR code for withdrawing any amount, but will ask for confirmation in the @{{ .BotName }} chat.
<code>/pay</code>, when sent as a reply to another message containing an invoice (for example, in a group), asks privately if you want to pay it.
    `,

	SENDHELP: `Sends satoshis to other Telegram users. The receiver is notified on his chat with @{{ .BotName }}. If the receiver has never talked to the bot or have blocked it he can't be notified, however. In that case you can cancel the transaction afterwards in the /transactions view.

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
	GIVEAWAYSATSGIVENPUBLIC: "{{.Sats}} sat given from {{.From}} to {{.To}}.{{if .ClaimerHasNoChat}} To manage your funds, start a conversation with @{{.BotName}}.{{end}}",
	CLAIMFAILED:             "Failed to claim {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:           "Claim",
	GIVEAWAYMSG:             "{{.User}} is giving {{.Sats}} sat away!",

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
	HIDDENREVEALBUTTON:   `{{.Sats}} to reveal {{if .Public}} in-place{{else }} privately{{end}}. {{if gt .Crowdfund 1}}Crowdfunding: {{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Revealers allowed: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "A message is hidden here. {{.Sats}} sat needed to unlock.",
	HIDDENWITHID:         "Message hidden with id <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Will be revealed publicly once {{.Message.Crowdfund}} people pay {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Will be revealed privately to the first {{.Message.Times}} payers{{else if .Message.Public}}Will be revealed publicly once one person pays {{.Message.Satoshis}}{{else}}Will be revealed privately to any payer{{end}}.",
	HIDDENSOURCEMSG:      "Hidden message <code>{{.Id}}</code> revealed by {{.Revealers}}. You've got {{.Sats}} sat.",
	HIDDENREVEALMSG:      "{{.Sats}} sat paid to reveal the message <code>{{.Id}}</code>.",
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

/bitflash_100000_3NRnMC5gVug7Mb4R3QHtKUcp27MAKAPbbJ buys an onchain transaction to the given address using bitflash.club's shared fee feature. Will ask for confirmation.
/bitflash_orders lists your previous transactions.
    `,

	MICROBETBETHEADER:           "<b>[Microbet]</b> Bet on one of these predictions:",
	MICROBETPAIDBUTNOTCONFIRMED: "<b>[Microbet]</b> Paid, but bet not confirmed. Huge Microbet bug?",
	MICROBETPLACING:             "<b>[Microbet]</b> Placing bet on <b>{{.Bet.Description}} ({{if .Back}}back{{else}}lay{{end}})</b>.",
	MICROBETPLACED:              "<b>[Microbet]</b> Bet placed!",
	MICROBETLIST: `
<b>[Microbet]</b> Your bets
{{range .Bets}}<code>{{.Description}}</code> {{if .UserBack}}{{.UserBack}}/{{.Backers}} × {{.Layers}}{{else}}{{.Backers}} × {{.UserLay}}/{{.Layers}}{{end}} <code>{{.Amount}}</code> <i>{{if .Canceled}}canceled{{else if .Closed}}{{if .WonAmount}}won {{.AmountWon}}{{else}}lost {{.AmountLost}}{{end}}{{else}}open{{end}}</i>
{{else}}
<i>~ no bets were ever made. ~</i>
{{end}}
    `,
	MICROBETBALANCE: "<b>[Microbet]</b> balance: <i>{{.Balance}} sat</i>",
	MICROBETHELP: `
<a href="https://microbet.fun/">Microbet</a> is a simple service that allows people to bet against each other on sports games results. The bet price is fixed and the odds are calculated considering the amount of back versus lay bets. There's a 1% fee on all withdraws.

/microbet displays all open bet markets so you can yours.
/microbet_bets shows your bet history.
/microbet_balance displays your balance.
/microbet_withdraw withdraws all your balance.
    `,

	BITREFILLINVENTORYHEADER: `<b>[Bitrefill]</b> Choose your provider:`,
	BITREFILLPACKAGESHEADER:  `<b>[Bitrefill]</b> Choose your <i>{{.Item}}</i> card{{if .ReplyCustom}} (or reply with a custom value){{end}}:`,
	BITREFILLNOPROVIDERS:     `<b>[Bitrefill]</b> No providers found.`,
	BITREFILLCONFIRMATION:    `<b>[Bitrefill]</b> Really buy a <i>{{.Package.Value}} {{.Item.Currency}}</i> card at <b>{{.Item.Name}}</b> for <i>{{.Sats}} sat</i> ({{dollar .Sats}})?`,
	BITREFILLFAILEDSAVE:      "<b>[Bitrefill]</b> Your order <code>{{.OrderId}}</code> was paid for, but not saved. Please report: {{.Err}}",
	BITREFILLPURCHASEDONE: `<b>[Bitrefill]</b> Your order <code>{{.OrderId}}</code> was purchased successfully.
{{if .Info.LinkInfo}}
Link: <a href="{{.Info.LinkInfo.Link}}">{{.Info.LinkInfo.Link}}</a>
Instructions: <i>{{.Info.LinkInfo.Other}}</i>
{{else if .Info.PinInfo}}
PIN: <code>{{.Info.PinInfo.Pin}}</code>
Instructions: <i>{{.Info.PinInfo.Instructions}}</i>
<i>{{.Info.PinInfo.Other}}</i>
{{end}}
    `,
	BITREFILLPURCHASEFAILED: "<b>[Bitrefill]</b> Your order was paid for, but Bitrefill encountered an error when trying to fulfill it: <i>{{.ErrorMessage}}</i>. Please report this so we can ask Bitrefill what to do.",
	BITREFILLCOUNTRYSET:     "<b>[Bitrefill]</b> Country set to {{if .CountryCode}}<code>{{.CountryCode}}</code>{{else}}none{{end}}.",
	BITREFILLINVALIDCOUNTRY: "<b>[Bitrefill]</b> Invalid country <code>{{.CountryCode}}</code>. The countries available are{{range .Available}} <code>{{.}}</code>{{end}}.",
	BITREFILLHELP: `
<a href="https://www.bitrefill.com/">Bitrefill</a> is the biggest Lightning-enabled gift-card and phone refill store in the world. If you want to buy real-world stuff with Lightning, this should be your first stop.

To buy a gift card, use the /bitrefill command followed by the name of the place you're looking for. To refill a phone, do the same but also append your phone (prefixed with the phone country code) at the end. Optionally you can also set your country with <code>/bitrefill country</code> so you'll only get suggestions available in your country and skip having to click through a bunch of different Amazons, for example.

<code>/bitrefill country AR</code> will set your default country to Argentina.
<code>/bitrefill country ''</code> will unset your default country.
<code>/bitrefill nextel +5411971732181</code> will display options to refill the given phone number of the operator Nextel.
<code>/bitrefill amazon</code> will display options of gift cards of various sizes you can buy on Amazon.

You may not find all the providers available in the <a href="https://www.bitrefill.com/">official Bitrefill website</a> through the bot and maybe other things are different here. But the prices are the same.
    `,

	SATELLITEFAILEDTOSTORE:     "<b>[satellite]</b> Failed to store satellite order data. Please report: {{.Err}}",
	SATELLITEFAILEDTOGET:       "<b>[satellite]</b> Failed to get stored satellite data: {{.Err}}",
	SATELLITEPAID:              "<b>[satellite]</b> Transmission <code>{{.UUID}}</code> queued!",
	SATELLITEFAILEDTOPAY:       "<b>[satellite]</b> Failed to pay for transmission.",
	SATELLITETRANSMISSIONERROR: "<b>[satellite]</b> Error making transmission: {{.Err}}",
	SATELLITELIST: `
<b>[Satellite]</b> Your transmissions
{{range .Orders}}📡 <code>{{.UUID}}</code> <i>{{.Status}}</i> <code>{{.MessageSize}}b</code> <code>{{printf "%.62" .BidPerByte}} msat/b</code> <i>{{.Time}}</i>
{{else}}
<i>No transmissions made yet.</i>
{{end}}
    `,
	SATELLITEHELP: `
The <a href="https://blockstream.com/satellite/">Blockstream Satellite</a> is a service that broadcasts Bitcoin blocks and other transmissions to the entire planet. You can transmit any message you want and pay with some satoshis.

<code>/satellite 13 'hello from the satellite! vote trump!'</code> queues that transmission to the satellite with a bid of 13 satoshis.
/satellite_transmissions lists your transmissions.
    `,

	FUNDBTCFINISH: "Finish your order by sending <code>{{.Order.Price}} BTC</code> to <code>{{.Order.Address}}</code>.",
	FUNDBTCHELP: `
Provided by <a href="https://golightning.club/">golightning.club</a>, this is the cheapest way to get your on-chain funds to Lightning, at just 99 satoshi per order. First you specify how much you want to receive, then you send money plus fees to the provided BTC address. Done.

/fundbtc_1000000 creates an order to transfer 0.01000000 BTC from an on-chain address to your bot balance.
    `,

	BITCLOUDSHELP: `
<a href="https://bitclouds.sh">bitclouds.sh</a> is a programmable VPS platform specialized in Bitcoin stuff. You can get normal VPSes, dedicated Bitcoin Core or batteries-included c-lightning instances all for 66 sat/h. There's no cheaper price than this and no excuses for not having your own Lightning node or not running your Bitcoin or Lightning app!

/bitclouds will let you see status for your active hosts.
/bitclouds_create will prompt your with the available images to create a host.
<code>/bitclouds topup &lt;sats&gt;</code> will topup your host or prompt you if you have more than one.

Also @{{.BotName}} will remind you to topup your hosts when they're running low on hour balance.
    `,
	BITCLOUDSCREATEHEADER: "<b>[bitclouds]</b> Choose your image:",
	BITCLOUDSCREATED: `<b>[bitclouds]</b> Your <i>{{.Image}}</i> host <code>{{.Host}}</code> is ready!
{{with .Status}}
  {{if .SSHPwd}}<b>ssh access:</b>
  <pre>ssh-copy-id -p{{.SSHPort}} {{.SSHUser}}@{{.IP}}
# type password: {{.SSHPwd}}
ssh -p{{.SSHPort}} {{.SSHUser}}@{{.IP}}</pre>{{end}}
  {{with .Sparko}}<b>Visit your <a href="{{.}}">Spark wallet</a> or call c-lightning RPC from the external world:</b>
<b>Call c-lightning RPC from the external world:</b>
  <pre>curl -kX POST {{.}}/rpc -d '{"method": "getinfo"}' -H 'X-Access: grabyourkeyinside'</pre>{{end}}
  {{if .RPCPwd}}<b>Call Bitcoin Core RPC:</b>
  <pre>bitcoin-cli -rpcport={{.RPCPort}} -rpcuser={{.RPCUser}} -rpcpassword={{.RPCPwd}} getblockchaininfo</pre>{{end}}
  Hours left in balance: <b>{{.HoursLeft}}</b>
{{end}}
    `,
	BITCLOUDSSTOPPEDWAITING: "<b>[bitclouds]</b> Timed out while waiting for your bitclouds.sh host <code>{{.Host}}</code> to be ready, call /bitclouds_status_{{.EscapedHost}} in a couple of minutes -- if it still doesn't work please report this issue along with the payment proof.",
	BITCLOUDSNOHOSTS:        "<b>[bitclouds]</b> No hosts found in your account. Maybe you want to /bitclouds_create one?",
	BITCLOUDSHOSTSHEADER:    "<b>[bitclouds]</b> Choose your host:",
	BITCLOUDSSTATUS: `<b>[bitclouds]</b> Host <code>{{.Host}}</code>:
{{with .Status}}
  Status: <i>Subscribed</i>
  Balance: <i>{{.HoursLeft}} hours left</i>
  IP: <code>{{.IP}}</code>
  {{if .UserPort }}App port: <code>{{.UserPort}}</code>
  {{end}}{{if .SSHPort}}SSH: <code>ssh -p{{.SSHPort}} {{.SSHUser}}@{{.IP}}</code>
  {{end}}{{with .Sparko}}<a href="{{.}}">Sparko</a>: <code>curl -k -X POST {{.}}/rpc -d '{"method": "getinfo"}' -H 'X-Access: grabyourkeyinside'</code>
  {{end}}{{if .RPCPwd}}Bitcoin Core: <code>bitcoin-cli -rpcconnect={{.IP}} -rpcport={{.RPCPort}} -rpcuser={{.RPCUser}} -rpcpassword={{.RPCPwd}} getblockchaininfo</code>
  {{end}}
{{end}}
    `,
	BITCLOUDSREMINDER: `<b>[bitclouds]</b> {{if .Alarm}}⚠{{else}}⏰{{end}} Bitclouds host <code>{{.Host}}</code> is going to expire in {{if .Alarm}}<b>{{.TimeToExpire}}</b> and <i>everything is going to be deleted</i>!{{else}}{{.TimeToExpire}}.{{end}}

{{if .Alarm}}⚠⚠⚠⚠⚠

{{end}}Use /bitclouds_topup_{{.Sats}}_{{.EscapedHost}} to give it one week more!
    `,

	QIWIHELP: `
Transfer your satoshis to your <a href="https://qiwi.com/">Qiwi</a> account instantly. Powered by @lntorubbot.

<code>/qiwi 50 rub to 777777777</code> sends the equivalent of 50 rubles to 77777777.
<code>/qiwi default 999999999</code> sets 999999999 as your default account.
<code>/qiwi 10000 sat</code> sends 10000 sat as rubles to your default account.
/qiwi_default shows your default account.
/qiwi_list shows your past transactions.
    `,
	YANDEXHELP: `
Transfer your satoshis to your <a href="https://money.yandex.ru/">Yandex.Money</a> account instantly. Powered by @lntorubbot.

<code>/yandex 50 rub to 777777777</code> sends the equivalent of 50 rubles to 77777777.
<code>/yandex default 999999999</code> sets 999999999 as your default account.
<code>/yandex 10000 sat</code> sends 10000 sat as rubles to your default account.
/yandex_default shows your default account.
/yandex_list shows your past transactions.
    `,
	LNTORUBCONFIRMATION:  "Sending <i>{{.Sat}} sat ({{.Rub}} rubles)</i> to <b>{{.Type}}</b> account <code>{{.Target}}</code>. Is that ok?",
	LNTORUBFULFILLED:     "<b>[{{.Type}}]</b> Transfer <code>{{.OrderId}}</code> finished.",
	LNTORUBCANCELED:      "<b>[{{.Type}}]</b> Transfer <code>{{.OrderId}}</code> canceled.",
	LNTORUBFIATERROR:     "<b>[{{.Type}}]</b> Error sending out the rubles. Please report this issue with the order id <code>{{.OrderId}}</code>.",
	LNTORUBMISSINGTARGET: "<b>[{{.Type}}]</b> You didn't specify a destination and there isn't a default destination specified!",
	LNTORUBDEFAULTTARGET: `<b>[{{.Type}}]</b> Default target: {{.Target}}`,
	LNTORUBORDERLIST: `<b>[{{.Type}}]</b>
{{range .Orders}}<i>{{.Sat}} sat ({{.Rub}} rub)</i> to <code>{{.Target}}</code> at <i>{{.Time}}</i>
{{else}}
<i>~ no sats were ever exchanged. ~</i>
{{end}}
    `,

	GIFTSHELP: `
<a href="https://lightning.gifts/">Lightning Gifts</a> is the best way to send satoshis as gifts to people. A simple service, a simple URL, no vendor lock-in and <b>no fees</b>.

By generating your gifts on @{{ .BotName }} you can keep track of the ones that were redeemed and the ones that weren't.

/gifts lists the gifts you've created.
/gifts_1000 creates a gift voucher of 1000 satoshis.
    `,
	GIFTSCREATED:    "<b>[gifts]</b> Gift created. To redeem visit <code>https://lightning.gifts/redeem/{{.OrderId}}</code>.",
	GIFTSFAILEDSAVE: "<b>[gifts]</b> Failed to save your gift. Please report: {{.Err}}",
	GIFTSLIST: `<b>[gifts]</b>
{{range .Gifts}}- <a href="https://lightning.gifts/redeem/{{.OrderId}}">{{.Amount}}sat</a> {{if .Spent}}redeemed on <i>{{.WithdrawDate}}</i> by {{.RedeemerURL}}{{else}}not redeemed yet{{end}}
{{else}}
<i>~ no gifts were ever given. ~</i>
{{end}}
    `,
	GIFTSSPENTEVENT: `<b>[gifts]</b> Gift redeemed!

Your {{.Amount}} sat gift <code>{{.Id}}</code> was redeemed{{if .Description}} from an invoice described as
<i>{{.Description}}</i>{{end}}.
    `,

	PAYWALLHELP: `
<a href="https://paywall.link/">paywall.link</a> is a Paywall Generator. It allows you to sell digital goods (files, articles, music, videos, any form of content that can be published on the open web) by simply wrapping their URLs in a paywall.

By generating your paywalls on @{{ .BotName }} you can keep track of them all without leaving Telegram and get information on how much of each you've sold.

/paywall will list all your paywalls.
<code>/paywall https://mysite.com/secret-content 230 'access my secret content'</code> will create a paywall for a secret content with a price of 230 satoshis.
/paywall_balance will show your paywall.link balance and ask you if you want to withdraw it.
/paywall_withdraw will just withdraw all your paywall.link balance to your @{{ .BotName }} balance.
    `,
	PAYWALLBALANCE: "<b>[paywall]</b> Balance: <i>{{.Balance}} sat</i>",
	PAYWALLCREATED: `<b>[paywall]</b> Paywall created: {{.Link.LndValue}} sat for <a href="{{.Link.DestinationURL}}">{{.Link.DestinationURL}}</a>: <code>https://paywall.link/to/{{.Link.ShortURL}}</code>: <i>{{.Link.Memo}}</i>`,
	PAYWALLLISTLINKS: `<b>[paywall]</b>
{{range .Links}}- <code>{{.LndValue}} sat</code> <a href="https://paywall.link/to/{{.ShortURL}}">{{.DestinationURL}}</a>: <i>{{.Memo}}</i>
{{else}}
<i>~ no paywalls were ever built. ~</i>
{{end}}
    `,
	PAYWALLPAIDEVENT: `<b>[paywall]</b> New click!
Someone just paid {{.Sats}} sat at your paywall <a href="{{.Link}}">{{.Memo}}</a> for <i>{{.Destination}}</i>.
    `,

	POKERDEPOSITFAIL:  "<b>[Poker]</b> Failed to deposit: {{.Err}}",
	POKERWITHDRAWFAIL: "<b>[Poker]</b> Failed to withdraw: {{.Err}}",
	POKERSECRETURL:    `<a href="{{.URL}}">Your personal secret Poker URL is here, never share it with anyone.</a>`,
	POKERBALANCE:      "<b>[Poker]</b> Balance: {{.Balance}}",
	POKERSTATUS: `
<b>[Poker]</b>
Players online: {{.Players}}
Active Tables: {{.Tables}}
Satoshis in play: {{.Chips}}

/poker_play to play here!
/poker_url to play in a browser window!
    `,
	POKERNOTIFY: `
<b>[Poker]</b> There are {{.Playing}} people playing {{if ne .Waiting 0}}and {{.Waiting}} waiting to play {{end}}poker right now{{if ne .Sats 0}} with a total of {{.Sats}} in play{{end}}!

/poker_status to double-check!
/poker_play to play here!
/poker_url to play in a browser window!
    `,
	POKERNOTIFYFRIEND: `
<b>[Poker]</b> {{.FriendName}} has sitted in a poker table!

/poker_status to double-check!
/poker_play to play here!
/poker_url to play in a browser window!
    `,
	POKERSUBSCRIBED: "You are available to play poker for the next {{.Minutes}} minutes.",
	POKERHELP: `<a href="https://lightning-poker.com/">Lightning Poker</a> is the first and simplest multiplayer live No-Limit Texas Hold'em Poker game played directly with satoshis. Just join a table and start staking sats.

By playing from an account tied to your @{{ .BotName }} balance you can just sit on a table and your poker balance will be automatically refilled from your @{{ .BotName }} account, with minimal friction.

/poker_deposit_10000 puts 10000 satoshis in your poker bag.
/poker_balance shows how much you have there.
/poker_withdraw brings all the money back to the bot balance.
/poker_status tells you how active are the poker tables right now.
/poker_url displays your <b>secret</b> game URL which you can open from any browser and gives access to your bot balance.
/poker_play displays the game widget.
/poker_watch_120 will put you in a subscribed state on the game for 2 hours and notify other subscribed people you are waiting to play. You'll be notified whenever there were people playing. If you join a game you'll be unsubscribed.
    `,

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
Images and videos are priced as if they were 300 characters.

To broadcast an ad you must send a message to the bot that will be your ad contents, then reply to it using <code>/sats4ads broadcast ...</code> as described. You can use <code>--max-rate=500</code> and <code>--skip=0</code> to have better control over how your message is going to be broadcasted. These are the defaults.

/sats4ads_on_15 puts your account in ad-listening mode. Anyone will be able to publish messages to you for 15 msatoshi-per-character. You can adjust that price.
/sats4ads_off turns off your account so you won't get any more ads.
/sats4ads_rates shows a breakdown of how many nodes are at each price level. Useful to plan your ad budget early.
/sats4ads_broadcast_1000 broadcasts an ad. The last number is the maximum number of satoshis that will be spend. Cheaper ad-listeners will be preferred over more expensive ones. Must be called in a reply to another message, the contents of which will be used as the ad text.
    `,
	SATS4ADSTOGGLE:    `<b>[sats4ads]</b> {{if .On}}Seeing ads and receiving {{printf "%.3f" .Sats}} sat per character.{{else}}You won't see any more ads.{{end}}`,
	SATS4ADSBROADCAST: `<b>[sats4ads]</b> {{if .NSent}}Message broadcasted {{.NSent}} time{{s .NSent}} for a total cost of {{.Sats}} sat ({{dollar .Sats}}).{{else}}Couldn't find a peer to notify with the given parameters. /sats4ads_rates{{end}}`,
	SATS4ADSPRICETABLE: `<b>[sats4ads]</b> Quantity of users <b>up to</b> each pricing tier.
{{range .Rates}}<code>{{.UpToRate}} msat</code>: <i>{{.NUsers}} user{{s .NUsers}}</i>
{{else}}
<i>No one is registered to see ads yet.</i>
{{end}}
Each ad costs the above prices <i>per character</i> + <code>1 sat</code> for each user.
    `,
	SATS4ADSADFOOTER: `[sats4ads: {{printf "%.3f" .Sats}} sat]`,

	HELPHELP: "Shows full help or help about specific command.",

	STOPHELP: "The bot stops showing you notifications.",

	CONFIRMINVOICE: `
{{.Sats}} sat ({{dollar .Sats}})
<i>{{.Desc}}</i>
<b>Hash</b>: {{.Hash}}
<b>Node</b>: {{.Node}} ({{.Alias}})

Pay the invoice described above?
    `,
	FAILEDDECODE: "Failed to decode invoice: {{.Err}}",
	NOINVOICE:    "Invoice not provided.",
	BALANCEMSG: `
<b>Full Balance</b>: {{printf "%.3f" .Sats}} sat ({{dollar .Sats}})
<b>Usable Balance</b>: {{printf "%.3f" .Usable}} sat ({{dollar .Usable}})
<b>Total received</b>: {{printf "%.3f" .Received}} sat
<b>Total sent</b>: {{printf "%.3f" .Sent}} sat
<b>Total fees paid</b>: {{printf "%.3f" .Fees}} sat

/balance_apps
/transactions
    `,
	TAGGEDBALANCEMSG: `
<b>Total of</b> <code>received - spent</code> <b>on internal and third-party</b> /apps<b>:</b>

{{range .Balances}}<code>{{.Tag}}</code>: <i>{{printf "%.0f" .Balance}} sat</i>  ({{dollar .Balance}})
{{else}}
<i>No tagged transactions made yet.</i>
{{end}}
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
	INVALIDAMOUNT:      "Invalid amount: {{.Amount}}",
	USERSENTTOUSER:     "{{.Sats}} sat sent to {{.User}}{{if .ReceiverHasNoChat}} (couldn't notify {{.User}} as they haven't started a conversation with the bot){{end}}",
	USERSENTYOUSATS:    "{{.User}} has sent you {{.Sats}} sat ({{dollar .Sats}}){{if .BotOp}} on a {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:   "Someone has sent you {{.Sats}} sat ({{dollar .Sats}}).",
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
	WELCOME: `
Welcome. Your account is created. You're now able to move Bitcoin into, from and inside Telegram. Please remember that we can't guarantee your funds in case we lose funds due to software bug or malicious hacker attacks. Don't keep a balance here greater than what you're willing to lose.

With that said, this bot is pretty safe.

For any questions or just to say hello you can join us at @lntxbot_dev (warning: there may be an entrance fee payable in satoshis).
    `,
	WRONGCOMMAND:    "Could not understand the command. /help",
	RETRACTQUESTION: "Retract unclaimed tip?",
	RECHECKPENDING:  "Recheck pending payment?",
	TXNOTFOUND:      "Couldn't find transaction {{.HashFirstChars}}.",
	TXINFO: `{{.Txn.Icon}} <code>{{.Txn.Status}}</code> {{.Txn.PeerActionDescription}} on {{.Txn.TimeFormat}} {{if .Txn.IsUnclaimed}}(💤y unclaimed){{end}}
<i>{{.Txn.Description}}</i>{{if not .Txn.TelegramPeer.Valid}}
{{if .Txn.Payee.Valid}}<b>Payee</b>: {{.Txn.PayeeLink}} ({{.Txn.PayeeAlias}}){{end}}
<b>Hash</b>: {{.Txn.Hash}}{{end}}{{if .Txn.Preimage.String}}
<b>Preimage</b>: {{.Txn.Preimage.String}}{{end}}
<b>Amount</b>: {{.Txn.Amount}} sat ({{dollar .Txn.Amount}})
{{if not (eq .Txn.Status "RECEIVED")}}<b>Fee paid</b>: {{.Txn.FeeSatoshis}}{{end}}
{{.LogInfo}}
    `,
	TXLIST: `<b>{{if .Offset}}Transactions from {{.From}} to {{.To}}{{else}}Latest {{.Limit}} transactions{{end}}</b>
{{range .Transactions}}<code>{{.StatusSmall}}</code> <code>{{.PaddedSatoshis}}</code> {{.Icon}} {{.PeerActionDescription}}{{if not .TelegramPeer.Valid}}<i>{{.Description}}</i>{{end}} <i>{{.TimeFormatSmall}}</i> /tx{{.HashReduced}}
{{else}}
<i>No transactions made yet.</i>
{{end}}
    `,

	TUTORIALWALLET: `
@{{.BotName}} is a Lightning wallet that works from your Telegram account.

You can use it to pay and receive Lightning invoices, it keeps track of your balances and a history of your transactions.

It also supports <a href="https://github.com/btcontract/lnurl-rfc/blob/master/spec.md#3-lnurl-withdraw">lnurl-withdraws</a> to and from other places, handles pending and failed transactions smoothly, does <a href="https://twitter.com/VNumeris/status/1148403575820709890">QR code scanning</a> (although for that you have to take a picture of the QR code with your Telegram app and that may fail depending on your phone's camera, patience and luck) and other goodies.

With @{{ .BotName }} you're well equipped for doing online stuff on the Lightning Network.
    `,
	TUTORIALBLUE: `
Although it works, for real-world usage opening a Telegram chat and pasting invoices can be a pain.

For usage on the streets you can import your @{{ .BotName }} funds on <a href="https://bluewallet.io/">BlueWallet</a>. You don't need to keep your on-chain Bitcoin there, nor create a default Lightning wallet, you just have to type /bluewallet here to get an import URL and paste it there on their import screen.

Everything you do on <a href="https://bluewallet.io/">BlueWallet</a> afterwards will be reflected in the bot screen and vice-versa (you'll get notifications for payments made and received from <a href="https://bluewallet.io/">BlueWallet</a> on your Telegram, but not the opposite).
    `,
	TUTORIALAPPS: `
Thanks to some background magic we have in place you can seamlessly interact with internal and third-party apps from the comfort of your @{{ .BotName }} chat, using your balance automatically -- so no more selecting options, manually typing amounts (or, worse, invoices) on websites before actually making transactions.

These are the services we currently support:

📢 /sats4ads -- get paid to see ads, pay to broadcast ads. /help_sats4ads
☁️ /bitclouds -- create and manage VPSes, Bitcoin and Lightning nodes as-a-service. /help_bitclouds
⚽ /microbet -- place bets on microbet.fun and withdraw your balance with a single click. /help_microbet
♠️ /poker -- play lightning-poker.com by automatically paying table buy-ins and keeping a unified balance. /help_poker
🧱 /paywall -- create paywalls on paywall.link, get notified whenever someone pays, withdraw easily. /help_paywall
🎁 /gifts -- create  a withdrawable link on lightning.gifts you can send to friends, get notified when they are spent, don't lose the redeem links. /help_gifts
📡 /satellite -- send messages from the space using the Blockstream Satellite. /help_satellite
🎲 /coinflip -- create a winner-takes-all fair lottery with satoshis at stake on a group chat. /help_coinflip
🎁 /giveaway  and /giveflip -- generate a message that gives money from your to the first person to click or to the lottery winner. /help_giveaway /help_giveflip
📢 /fundraise -- many people contribute to a single person, for good causes. /help_fundraise
📲 /bitrefill -- buy gift cards and refill phones. /help_bitrefill
💸 /yandex and /qiwi -- send satoshis to an yandex.money or qiwi.com account as rubles with the best exchange rate.  /help_yandex /help_qiwi 
⛓️ /fundbtc -- send satoshis from your on-chain Bitcoin wallet to your @{{ .BotName }} balance, powered by golightning.club. /help_fundbtc

Read more in the /help page for each app.
    `,
}
