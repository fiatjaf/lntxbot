package t

var EN = map[Key]string{
	NO:         "Нет",
	YES:        "Да",
	CANCEL:     "Отмена",
	CANCELED:   "Отменено.",
	COMPLETED:  "Выполнено!",
	CONFIRM:    "Подтвердить",
	FAILURE:    "Сбой.",
	PROCESSING: "Обрабатываю...",
	WITHDRAW:   "Вывести?",
	ERROR:      "Ошибка{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:   "Проверка...",
	TXCANCELED: "Транзакция отменена.",
	UNEXPECTED: "Неожиданная ошибка: пожалуйста, обратитесь в поддержку.",

	CALLBACKWINNER:  "Победитель: {{.Winner}}",
	CALLBACKERROR:   "{{.BotOp}} ошибка{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED: "{{.BotOp}} время истекло.",
	CALLBACKATTEMPT: "Ищу маршрут.",
	CALLBACKSENDING: "Отправляю платёж.",

	INLINEINVOICERESULT:  "Счёт на {{.Sats}} сат.",
	INLINEGIVEAWAYRESULT: "Раздать {{.Sats}}",
	INLINEGIVEFLIPRESULT: "Раздаёт {{.Sats}} сат одному из {{.MaxPlayers}} участников",
	INLINECOINFLIPRESULT: "Лотерея с входным платежом {{.Sats}} сат для {{.MaxPlayers}} участников",
	INLINEHIDDENRESULT:   "{{.HiddenId}} ({{if gt .Message.Crowdfund 1}}crowd:{{.Message.Crowdfund}}{{else if gt .Message.Times 0}}priv:{{.Message.Times}}{{else if .Message.Public}}pub{{else}}priv{{end}}): {{.Message.Content}}",

	LNURLINVALID: "Неверный lnurl: {{.Err}}",
	LNURLFAIL:    "Ошибка при выводе через lnurl: {{.Err}}",

	USERALLOWED:       "Счёт оплачен. {{.User}} допущен.",
	SPAMFILTERMESSAGE: "Привет, {{.User}}. У вас 15мин, чтобы оплатить счёт на {{.Sats}} сат если вы хотите остаться в этой группе:",

	PAYMENTFAILED: "Платёж не состоялся. /log{{.ShortHash}}",
	PAIDMESSAGE: `Оплачено <b>{{.Sats}} сат</b> (+ {{.Fee}} комиссия). 

<b>Hash:</b> {{.Hash}}
{{if .Preimage}}<b>Proof:</b> {{.Preimage}}{{end}}

/tx{{.ShortHash}}`,
	DBERROR:             "Ошибка базы данных: не могу отметить транзакцию как не обрабатывающуюся.",
	INSUFFICIENTBALANCE: "Недостаточный баланс для {{.Purpose}}. Необходимо {{.Sats}}.0f сат больше.",
	TOOSMALLPAYMENT:     "Это слишком мало, пожалуйста, начните {{.Purpose}} от 40 сат.",

	PAYMENTRECEIVED:      "Платёж получен: {{.Sats}}. /tx{{.Hash}}.",
	FAILEDTOSAVERECEIVED: "Платёж получен, но не сохранён в базе данных. Пожалуйста, сообщите о проблеме: <code>{{.Label}}</code>, hash: <code>{{.Hash}}</code>",

	SPAMMYMSG:    "{{if .Spammy}}Теперь эта группа будет спамиться. {{else}}Больше спамить не буду.{{end}}",
	TICKETMSG:    "Новые участники должны заплатить {{.Sat}} сат (убедитесь, что вы установили @{{.BotName}} администратором, чтобы это работало).",
	FREEJOIN:     "К этой группе теперь можно присоединиться свободно.",
	ASKTOCONFIRM: "Заплатить счёт выше?",

	HELPINTRO: `
<pre>{{.Help}}</pre>
Для более подробной информации по каждой команде пожалуйста введите <code>/help &lt;command&gt;</code>.
    `,
	HELPSIMILAR: "/{{.Method}} команда не найдена. Вы имели в виду /{{index .Similar 0}}?{{if gt (len .Similar) 1}} Или может быть /{{index .Similar 1}}?{{if gt (len .Similar) 2}} Возможно {{index .Similar 2}}?{{end}}{{end}}",
	HELPMETHOD: `
<pre>/{{.MainName}} {{.Argstr}}</pre>
{{.Help}}
{{if .HasInline}}
<b>Инлайн запрос</b>
Может быть также вызвана как <a href=\"https://core.telegram.org/bots/inline\">инлайн запрос</a> из группы или в персональном чате, где бот ещё не добавлен. Синтаксис команды похож, но сделан проще: <code>@{{.ServiceId}} {{.InlineExample}}</code>, затем пользователь должен подождать пока результат автоматического \"поиска\" не будет показан ему.
{{if .Aliases}}
<b>Алиасы:</b> <code>{{.Aliases}}</code>{{end}}
    `,

	// the "any" is here only for illustrative purposes. if you call this with 'any' it will
	// actually be assigned to the <сатoshis> variable, and that's how the code handles it.
	RECEIVEHELP: `Создаёт BOLT11 счёт с заданным значением сатоши. Полученные токены будут добавлены к вашему балансу в боте. Если вы не укажете количество, это будет счёт с открытым полем значения, в который может быть добавлено любое количество.",

<code>/receive_320_for_something</code> создаёт запрос на 320 сат с описанием "for something"
<code>/receive 100 за скрытые данные --preimage="0000000000000000000000000000000000000000000000000000000000000000"</code> создаёт счёт с заданным преимаджем (будьте осторожны, вы можете потерять деньги, используйте только если полностью уверены в том, что делаете).
    `,

	PAYHELP: `Расшифровывает BOLT11 счёт и спрашивает хотите ли вы его оплатить (иначе используйте /paynow). Будет получен такой же результат как если бы пользователь скопировал и вставил счёт в чат с ботом. Фото с изображением QR с зашифрованным счётом также работает (если картинка достаточно качественная).

Просто вставляя <code>lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> расшифровывает и печатает заданный счёт.  
<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> оплачивает счёт без подтверждения.
/withdraw_lnurl_3000 создаёт lnurl и QR код для вывода 3000 сатоши из <a href="https://lightning-wallet.com">совместимого кошелька</a> без запроса подтверждения.
/withdraw_lnurl создаёт lnurl и QR код для вывода любого количества, но запросит подтверждение.
<code>/pay</code>, когда отправлено как ответ на другое сообщение, содержащее счёт (например в групповом чате), спрашивает в чате с ботом, согласен ли пользователь оплатить счёт.
    `,

	SENDHELP: `Отправляет сатоши другим пользователям Telegram. Получатель оповещается в его чате с ботом. Если получатель ещё не запустил бот, или заблокировал его, он не может быть оповещён. В этом случае вы можете отменить транзакцию после, вызвав /transactions.

<code>/tip 100</code>, Если эта команда вызывается в ответ на сообщение в групповом чате, где бот добавлен, то автору сообщения будет начислено 100 сатоши.
<code>/send 500 @username</code> отправляет 500 сатоши пользователю Telegram @username.
<code>/send anonymously 1000 @someone</code> то же, что выше, Telegram пользователь @someone увидит только: "Кто-то отправил вам 1000 сатоши".
    `,

	BALANCEHELP: "Покажет вам текущий баланс в сатоши, плюс сумму всего, что вы получили и отправили внутри бота и сумму всех комиссий.",

	GIVEAWAYHELP: `Создаст кнопку в групповом чате. Первый, кто нажмёт на неё, получит сатоши.

/giveaway_1000: как только кто-либо нажмёт 'Получить' 1000 сатоши будут переведены кликеру.
    `,
	GIVEAWAYSATSGIVENPUBLIC: "{{.Sats}} сат подарены от {{.From}} пользователю {{.To}}.{{if .ClaimerHasNoChat}} Для управления своими токенами, начните диалог с @{{.BotName}}.{{end}}",
	CLAIMFAILED:             "Ошибка запроса {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:           "Получить",
	GIVEAWAYMSG:             "{{.User}} дарит {{.Sats}} сат!",

	COINFLIPHELP: `Запускает честную лотерею подбрасывания монетки с указанным количеством участников. Все платят такое же количество, как было указано в стоимости входа. Победитель получает всё. Токены перемещаются только в тот момент, когда запускается лотерея.

/coinflip_100_5: 5 участников необходимы для старта, победитель получит 500 сатоши (включая его собственные 100, поэтому чистым выигрышем он получит 400 сатоши).
    `,
	COINFLIPWINNERMSG:      "Вы победитель в подбросе монетки с призом {{.TotalSats}} сат. Проигравшие: {{.Senders}}.",
	COINFLIPGIVERMSG:       "Вы проиграли {{.IndividualSats}} в подбросе монетки. Победителем стал {{.Receiver}}.",
	COINFLIPAD:             "Заплатите {{.Sats}} получите шанс выиграть {{.Prize}}! Осталось {{.SpotsLeft}} из {{.MaxPlayers}} спотов!",
	COINFLIPJOIN:           "Присоединиться к розыгрышу!",
	CALLBACKCOINFLIPWINNER: "Победитель: {{.Winner}}",
	COINFLIPOVERQUOTA:      "Вы превысили квоту игры в монетку.",
	COINFLIPRATELIMIT:      "Пожалуйста, подождите 30 минут перед запуском нового раунда монетки.",

	GIVEFLIPHELP: `Начинает раздачу случайным методом, но, вместо подарка первому пользователю, который нажмёт на кнопку, количество будет разыграно между первыми x кликерами.

/giveflip_100_5: 5 участников необходимо 500 сатоши получит победитель от инициатора команды.
    `,
	GIVEFLIPMSG:       "{{.User}} раздаёт {{.Sats}} сат счастливчику из {{.Participants}} участников!",
	GIVEFLIPAD:        "{{.Sats}} будут разданы. Присоединись и получи шанс выиграть! Осталось {{.SpotsLeft}} из {{.MaxPlayers}} мест!",
	GIVEFLIPJOIN:      "Попробую выиграть!",
	GIVEFLIPWINNERMSG: "{{.Sender}} отправил(а) {{.Sats}} сат {{.Receiver}}. Ничего не получили пользователи: {{.Losers}}.{{if .ReceiverHasНетChat}} Для управления своими деньгами начните диалог с @{{.BotName}}.{{end}}",

	FUNDRAISEHELP: `Начинает краудфандинг с заранее определённым количеством участников и вкладом каждого. Если количество участников будет достигнуто, фандрайзинг будет актуализирован. Иначе он будет отменён через несколько часов.

<code>/fundraise 10000 8 @user</code>: Пользователь @user получит 80000 сатоши, если 8 человек присоединятся к компании.
    `,
	FUNDRAISEAD: `
Фандрайзинг {{.Fund}} в пользу {{.ToUser}}!
Необходимо участников: {{.Participants}}
Вклад каждого: {{.Sats}} сат
Присоединились: {{.Registered}}
    `,
	FUNDRAISEJOIN:        "Присоединяюсь!",
	FUNDRAISECOMPLETE:    "Фандрайзинг для {{.Receiver}} завершён!",
	FUNDRAISERECEIVERMSG: "Вы получили {{.TotalSats}} сат от пользователей {{.Senders}}",
	FUNDRAISEGIVERMSG:    "Вы пожертвовали {{.IndividualSats}} в пользу {{.Receiver}}.",

	BLUEWALLETHELP: `Показывает ваши регистрационные данные для импорта кошелька бота в BlueWallet. Вы можете использовать тот же аккаунт из обоих мест попеременно.

/bluewallet Печатает строчку вроде \"lndhub://&lt;login&gt;:&lt;password&gt;@&lt;url&gt;\", которая должна быть скопирована и вставлена в BlueWallet функцию импорта.
/bluewallet_refresh очищает предыдущий пароль и печатает новую строку. Вы должны ре-импортировать регистрационные данные в кошелёк BlueWallet после этого шага. Делайте это только в том случае, если предыдущие данные были скомпрометированы.
    `,
	BLUEWALLETPASSWORDUPDATEERROR: "Ошибка обновления пароля. Сообщите о ней: {{.Err}}",
	BLUEWALLETCREDENTIALS:         "<code>{{.Credentials}}</code>",

	HIDEHELP: `Скрывает сообщение, которое может быть открыто позже после оплаты. Специальный символ "~" используется для того, чтобы разделить сообщение для предпросмотра ("нажмите здесь, чтобы открыть секрет! ~ это секрет.")

<code>/hide 500 'совершенно секретное сообщение'</code> скрывает "совершенно секретное сообщение" и возвращает id. Позже можно выпустить приглашение к открытию сообщения с помощью /reveal &lt;id_скрытого_сообщения&gt;
<code>/hide 2500 'только богатеи смогут посмотреть это сообщение' ~ 'поздравляю! вы действительно богаты'</code>: в этом случае все потенциальные адресаты скрытого сообщения будут видеть часть перед символом "~" в общем доступе.

Любой пользователь может открыть любое скрытое сообщение (после уплаты), набрав <code>/reveal &lt;id_скрытого_сообщения&gt;</code> в своём приватном чате с ботом, но id известен только создателю сообщения, если он его не разгласил.

The basic way to share the message, however, is to click the "share" button and use the inline query in a group or chat. That will post the message preview to the chat along with a button people can click to pay and reveal.  You can control if the message will be revealed in-place for the entire group to see or privately just to the payer using the <code>--public</code> flag. By default it's private.

You can also control how many people will be allowed to reveal it privately using the <code>--revealers</code> argument or how many will be required to pay before it is revealed publicly with the <code>--crowdfund</code> argument.

<code>/hide 100 'three people have paid for this message to be revealed' --crowdfund 3</code>: the message will be revealed publicly once 3 people pay 100 сатoshis.
<code>/hide 321 'only 3 people can see this message' ~ "you're one among 3 privileged" --revealers 3</code>: the message will be revealed privately to the first 3 people who click.
    `,
	REVEALHELP: `Reveals a message that was previously hidden. The author of the hidden message is never disclosed. Once a message is hidden it is available to be revealed globally, but only by those who know its hidden id.

A reveal prompt can also be created in a group or chat by clicking the "share" button after you hide the message, then the standard message reveal rules apply, see /help_hide for more info.

<code>/reveal 5c0b2rh4x</code> creates a prompt to reveal the hidden message 5c0b2rh4x, if it exists.
    `,
	HIDDENREVEALBUTTON:   `{{.Sats}} to reveal {{if .Public}} in-place{{else }} privately{{end}}. {{if gt .Crowdfund 1}}Crowdfunding: {{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Revealers allowed: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "A message is hidden here. {{.Sats}} сат needed to unlock.",
	HIDDENWITHID:         "Message hidden with id <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Will be revealed publicly once {{.Message.Crowdfund}} people pay {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Will be revealed privately to the first {{.Message.Times}} payers{{else if .Message.Public}}Will be revealed publicly once one person pays {{.Message.Satoshis}}{{else}}Will be revealed privately to any payer{{end}}.",
	HIDDENSOURCEMSG:      "Hidden message <code>{{.Id}}</code> revealed by {{.Revealers}}. You've got {{.Sats}} сат.",
	HIDDENREVEALMSG:      "{{.Sats}} сат paid to reveal the message <code>{{.Id}}</code>.",
	HIDDENSTOREFAIL:      "Failed to store hidden content. Please report: {{.Err}}",
	HIDDENMSGNOTFOUND:    "Hidden message not found.",
	HIDDENSHAREBTN:       "Share in another chat",

	APPHELP: `
You can use the following bots without leaving your bot chat:

lightning-poker.com, multiplayer texas hold'em: /help_poker
microbet.fun, simple sports betting: /help_microbet
lightning.gifts, lightning vouchers: /help_gifts
bitflash.club, LN->BTC with batched transactions: /help_bitflash
golightning.club, BTC->LN cheap service: /help_golightning
Blockstream Satellite, messages from space: /help_сатellite
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

/app_bitflash_100000_3NRnMC5gVug7Mb4R3QHtKUcp27MAKAPbbJ buys an onchain transaction to the given address using bitflash.club's shared комиссия feature. Will ask for confirmation.
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
{{range .Bets}}<code>{{.Description}}</code> {{if .UserBack}}{{.UserBack}}/{{.Backers}} × {{.Layers}}{{else}}{{.Backers}} × {{.UserLay}}/{{.Layers}}{{end}} <code>{{.Amount}}</code> <i>{{if .Canceled}}canceled{{else if .Closed}}{{if .WonAmount}}won {{.AmountWon}}{{else}}lost {{.AmountLost}}{{end}}{{else}}open{{end}}</i>
{{else}}
<i>~ no bets were ever made. ~</i>
{{end}}
    `,
	MICROBETBALANCEERROR: "Error fetching Microbet balance: {{.Err}}",
	MICROBETBALANCE:      "<b>[Microbet]</b> balance: <i>{{.Balance}} сат</i>",
	MICROBETHELP: `
<a href="https://microbet.fun/">Microbet</a> is a simple service that allows people to bet against each other on sports games results. The bet price is fixed and the odds are calculated considering the amount of back versus lay bets. There's a 1% комиссия on all withdraws.

/app_microbet_bet displays all open bet markets so you can yours.
/app_microbet_bets shows your bet history.
/app_microbet_balance displays your balance.
/app_microbet_withdraw withdraws all your balance.
    `,

	SATELLITEFAILEDTOSTORE:     "Failed to store сатellite order data. Please report: {{.Err}}",
	SATELLITEFAILEDTOGET:       "Failed to get stored сатellite data: {{.Err}}",
	SATELLITEPAID:              "Transmission <code>{{.UUID}}</code> paid!",
	SATELLITEFAILEDTOPAY:       "Failed to pay for transmission.",
	SATELLITEBUMPERROR:         "Error bumping transmission: {{.Err}}",
	SATELLITEFAILEDTODELETE:    "Failed to delete сатellite order data. Please report: {{.Err}}",
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
<i>Нет transmissions made yet.</i>
{{end}}
    `,
	SATELLITEHELP: `
The <a href="https://blockstream.com/сатellite/">Blockstream Satellite</a> is a service that broadcasts Bitcoin blocks and other transmissions to the entire planet. You can transmit any message you want and pay with some сатoshis.

<code>/app сатellite 13 'hello from the сатellite! vote trump!'</code> queues that transmission to the сатellite with a bid of 13 сатoshis.
/app_сатellite_transmissions lists your transmissions.
    `,

	GOLIGHTNINGFAIL:   "<b>[GoLightning]</b> Failed to create order: {{.Err}}",
	GOLIGHTNINGFINISH: "<b>[GoLightning]</b> Finish your order by sending <code>{{.Order.Price}} BTC</code> to <code>{{.Order.Address}}</code>.",
	GOLIGHTNINGHELP: `
<a href="https://golightning.club/">GoLightning.club</a> is the cheapest way to get your on-chain funds to Lightning, at just 99 сатoshi per order. First you specify how much you want to receive, then you send money plus комиссияs to the provided BTC address. Done.

/app_golightning_1000000 creates an order to transfer 0.01000000 BTC from an on-chain address to your bot balance.
    `,

	GIFTSHELP: `
<a href="https://lightning.gifts/">Lightning Gifts</a> is the best way to send сатoshis as gifts to people. A simple service, a simple URL, no vendor lock-in and <b>no комиссияs</b>.

/app_gifts_1000 creates a gift voucher of 1000 сатoshis.
    `,
	GIFTSERROR:      "<b>[gifts]</b> Error: {{.Err}}",
	GIFTSCREATED:    "<b>[gifts]</b> Gift created. To redeem just visit <code>https://lightning.gifts/redeem/{{.OrderId}}</code>.",
	GIFTSFAILEDSAVE: "<b>[gifts]</b> Failed to save your gift. Please report: {{.Err}}",
	GIFTSLIST: `
<b>gifts</b>
{{range .Gifts}}- <a href="https://lightning.gifts/redeem/{{.OrderId}}">{{.Amount}}сат</a> {{if .Spent}}redeemed on <i>{{.WithdrawDate}}</i> by {{.RedeemerURL}}{{else}}not redeemed yet{{end}}
{{else}}
<i>~ no gifts were ever given. ~</i>
{{end}}
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

/app_poker_play to play here!
/app_poker_url to play in a browser window!
    `,
	POKERNOTIFY: `
<b>[Poker]</b> There are {{.Playing}} people playing {{if ne .Waiting 0}}and {{.Waiting}} waiting to play {{end}}poker right now{{if ne .Sats 0}} with a total of {{.Sats}} in play{{end}}!

/app_poker_status to double-check!
/app_poker_play to play here!
/app_poker_url to play in a browser window!
    `,
	POKERSUBSCRIBED: "You are available to play poker for the next {{.Minutes}} minutes.",
	POKERHELP: `<a href="https://lightning-poker.com/">Lightning Poker</a> is the first and simplest multiplayer live Нет-Limit Texas Hold'em Poker game played directly with сатoshis. Just join a table and start staking сатs.

By playing from an account tied to your bot balance you can just sit on a table and your poker balance will be automatically refilled from your bot account, with minimal friction.

/app_poker_deposit_10000 puts 10000 сатoshis in your poker bag.
/app_poker_balance shows how much you have there.
/app_poker_withdraw brings all the money back to the bot balance.
/app_poker_status tells you how active are the poker tables right now.
/app_poker_url displays your <b>secret</b> game URL which you can open from any browser and gives access to your bot balance.
/app_poker_play displays the game widget.
/app_poker_watch_120 will put you in a subscribed state on the game for 2 hours and notify other subscribed people you are waiting to play. You'll be notified whenever there were people playing.
    `,

	TOGGLEHELP: `Toggles bot features in groups on/off. In supergroups it can only be run by admins.

<code>/toggle ticket 10</code> starts charging a комиссия for all new entrants. Useful as an antispam measure. The money goes to the group owner.
<code>/toggle ticket</code> stops charging new entrants a комиссия. 
<code>/toggle spammy</code>: 'spammy' mode is off by default. When turned on, tip notifications will be sent in the group instead of only privately.
    `,

	HELPHELP: "Shows full help or help about specific command.",

	STOPHELP: "The bot stops showing you notifications.",

	CONFIRMINVOICE: `
{{.Sats}} сат ({{.USD}})
<i>{{.Desc}}</i>
<b>Hash</b>: {{.Hash}}
<b>Нетde</b>: {{.Нетde}} ({{.Alias}})
    `,
	FAILEDDECODE: "Failed to decode invoice: {{.Err}}",
	NOINVOICE:    "Invoice not provided.",
	BALANCEMSG: `
<b>Balance</b>: {{printf "%.3f" .Sats}} сат ({{.USD}})
<b>Total received</b>: {{printf "%.3f" .Received}} сат
<b>Total sent</b>: {{printf "%.3f" .Sent}} сат
<b>Total комиссияs paid</b>: {{printf "%.3f" .Fees}} сат
    `,
	// {{if ne .CoinflipBalance 0}}<b>Coinflip balance</b>: {{.CoinflipBalance}} сат ({{.CoinflipWins}} won, {{.CoinflipLoses}} lost)
	// {{end}}
	//     `,
	FAILEDUSER: "Failed to parse receiver name.",
	LOTTERYMSG: `
A lottery round is starting!
Entry комиссия: {{.EntrySats}} сат
Total participants: {{.Participants}}
Prize: {{.Prize}}
Registered: {{.Registered}}
    `,
	INVALIDPARTNUMBER:  "Invalid number of participants: {{.Number}}",
	INVALIDAMOUNT:      "Invalid amount: {{.Amount}}",
	USERSENTTOUSER:     "{{.Sats}} сат sent to {{.User}}{{if .ReceiverHasНетChat}} (couldn't notify {{.User}} as they haven't started a converсатion with the bot){{end}}",
	USERSENTYOUSATS:    "{{.User}} has sent you {{.Sats}} сат{{if .BotOp}} on a {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:   "Someone has sent you {{.Sats}} сат.",
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
	STOPNOTIFY:         "Нетtifications stopped.",
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
<b>Amount</b>: {{.Txn.Amount}} сат
{{if not (eq .Txn.Status "RECEIVED")}}<b>Fee paid</b>: {{.Txn.FeeSatoshis}}{{end}}
{{.LogInfo}}
    `,
	TXLIST: `<b>{{if .Offset}}Transactions from {{.From}} to {{.To}}{{else}}Latest {{.Limit}} transactions{{end}}</b>
{{range .Transactions}}<code>{{.StatusSmall}}</code> <code>{{.PaddedSatoshis}}</code> {{.Icon}} {{.PeerActionDescription}}{{if not .TelegramPeer.Valid}}<i>{{.Description}}</i>{{end}} <i>{{.TimeFormatSmall}}</i> /tx{{.HashReduced}}
{{else}}
<i>Нет transactions made yet.</i>
{{end}}
    `,
}
