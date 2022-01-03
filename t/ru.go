package t

var RU = map[Key]string{
	NO:         "Нет",
	YES:        "Да",
	CANCEL:     "Отмена",
	CANCELED:   "Отменено.",
	COMPLETED:  "Выполнено!",
	CONFIRM:    "Подтвердить",
	PAYAMOUNT:  `Оплатить {{.Sats | printf "%.15g"}}`,
	FAILURE:    "Сбой.",
	PROCESSING: "Обрабатываю...",
	WITHDRAW:   "Вывести?",
	ERROR:      "🔴 {{if .App}}#{{.App | lower}} {{end}}Ошибка{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:   "Проверка...",
	TXPENDING:  "Платёж в процессе, пожалуйста, проверьте позже.",
	TXCANCELED: "Транзакция отменена.",
	UNEXPECTED: "Неожиданная ошибка: пожалуйста, обратитесь в поддержку.",
	MUSTBEADMIN: "Эта команда должна быть использована администратором.",
	MUSTBEGROUP: "Эта команда должна использоваться в чате.",
	
	CALLBACKWINNER:  "Победитель: {{.Winner}}",
	CALLBACKERROR:   "{{.BotOp}} ошибка{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED: "{{.BotOp}} время истекло.",
	CALLBACKATTEMPT: "Ищу маршрут. /tx_{{.Hash}}",
	CALLBACKSENDING: "Отправляю платёж.",

	INLINEINVOICERESULT:  "Запрос платежа на {{.Sats}} сат.",
	INLINEGIVEAWAYRESULT: "Подарите {{.Sats}} сат {{if .Receiver}}@{{.Receiver}}{{else}}кому угодно{{end}}",
	INLINEGIVEFLIPRESULT: "Дарит {{.Sats}} сат одному из {{.MaxPlayers}} участников",
	INLINECOINFLIPRESULT: "Лотерея с входным платежом {{.Sats}} сат для {{.MaxPlayers}} участников",
	INLINEHIDDENRESULT:   "{{.HiddenId}} ({{if gt .Message.Crowdfund 1}}собрать:{{.Message.Crowdfund}}{{else if gt .Message.Times 0}}прив:{{.Message.Times}}{{else if .Message.Public}}пуб{{else}}прив{{end}}): {{.Message.Content}}",

	LNURLUNSUPPORTED: "Такой тип lnurl не поддерживается.",
	LNURLERROR:       `<b>{{.Host}}</b> lnurl ошибка: {{.Reason}}`,
	LNURLAUTHSUCCESS: `
lnurl-auth успех!

<b>Домен</b>: <i>{{.Host}}</i>
<b>Публичный Ключ</b>: <i>{{.PublicKey}}</i>
`,
	LNURLPAYPROMPT: `🟢 <code>{{.Domain}}</code> ожидает {{if .FixedAmount}}<i>{{.FixedAmount | printf "%.15g"}} сат</i>{{else}}значение между <i>{{.Min | printf "%.15g"}}</i> и <i>{{.Max | printf "%.15g"}} сат</i>{{end}} для:

<code>{{if .Long}}{{.Long | html}}{{else}}{{.Text | html}}{{end}}</code>{{if .WillSendPayerData}}

---

- Ваше имя и/или ключи авторизации будут отосланы получателю платежа.
- Для предотвращения этого используйте <code>/lnurl --anonymous &lt;lnurl&gt;</code>.
{{end}}

{{if not .FixedAmount}}<b>Ответьте числом (в сатоши, между <i>{{.Min | printf "%.15g"}}</i> и <i>{{.Max | printf "%.15g"}}</i>), для подтверждения.</b>{{end}}
    `,
	LNURLPAYPROMPTCOMMENT: `📨 <code>{{.Domain}}</code> ожидает комментарий.

<b>Для подтверждения платежа прикрепите сообщение</b>`,
	LNURLPAYAMOUNTSNOTICE: `<code>{{.Domain}}</code> ожидал {{if .Exact}}{{.Min | printf "%.3f"}}{{else if .NoMax}}минимум{{.Min | printf "%.0f"}}{{else}}между {{.Min | printf "%.0f"}} и {{.Max | printf "%.0f"}}{{end}} сат.`,
	LNURLPAYSUCCESS: `<code>{{.Domain}}</code> ответил:
{{.Text}}
{{if .DecipherError}}Ошибка расшифровки ({{.DecipherError}}):
{{end}}{{if .Value}}<pre>{{.Value}}</pre>
{{end}}{{if .URL}}<a href="{{.URL}}">{{.URL}}</a>{{end}}
    `,
	LNURLPAYMETADATA: `#lnurlpay метаданные:
<b>домен</b>: <i>{{.Domain}}</i>
<b>транзакция</b>: /tx_{{.HashFirstChars}}
    `,
	LNURLBALANCECHECKCANCELED: "Отменена автоматическая проверка баланса на {{.Service}}.",

	TICKETSET: "Новые участники должны оплатить инвойс {{.Sat}} сат (убедитесь, что вы установили @lntxbot в качестве администратора).",
	TICKETUSERALLOWED: "Билет оплачен. Пользователь {{.User}} допущен.",
	TICKETMESSAGE: `⚠️ {{.User}}, для входа в чат оплатите {{.Sats}} сат.

У вас 15 минут на совершение платежа, в противном случае вы будете исключены и попадёте в черный список на 1 день.
`,
	
	RENAMABLEMSG:      "Любой может сменить название чата за {{.Sat}} сат (убедитесь, что вы установили @lntxbot в качестве администратора).",
	RENAMEPROMPT:      "Заплатить <b>{{.Sats}} сат</b> за переименование группы в <i>{{.Name}}</i>?",
	GROUPNOTRENAMABLE: "Эту группу невозможно переименовать!",

	INTERNALPAYMENTUNEXPECTED: "Произошло что-то странное. Если это был внутренний запрос платежа, то платёж не состоится. Вероятно, запрос устарел или произошло что-то ещё. Если это внешний запрос, игнорируйте это предупреждение.",
	PAYMENTFAILED: "❌ Платёж не состоялся. /log_{{.ShortHash}}",
	PAIDMESSAGE: `✅ Оплачено <i>{{printf "%.15g" .Sats}} сат</i> ({{dollar .Sats}}) (+ <i>{{.Fee}}</i> комиссия). 

<b>Hash:</b> <code>{{.Hash}}</code>{{if .Preimage}}
<b>Proof:</b> <code>{{.Preimage}}</code>{{end}}

/tx_{{.ShortHash}} ⚡️ #tx`,
	OVERQUOTA:           "Вы превысили недельную квоту {{.App}}.",
	RATELIMIT:           "Пожалуйста, подождите 30 минут.",
	DBERROR:             "Ошибка базы данных: не могу отметить транзакцию как не обрабатывающуюся.",
	INSUFFICIENTBALANCE: `Недостаточный баланс для {{.Purpose}}. Необходимо на {{.Sats | printf "%.15g"}} сат больше.`,

	PAYMENTRECEIVED: `
      ⚡️ Платёж получен{{if .SenderName}} от <i>{{ .SenderName }}</i>{{end}}: {{.Sats}} сат ({{dollar .Sats}}). /tx_{{.Hash}}{{if .Message}} {{.Message | messageLink}}{{end}} #tx
      {{if .Comment}}
📨 <i>{{.Comment}}</i>
      {{end}}
    `,
	FAILEDTOSAVERECEIVED: "Платёж получен, но не сохранён в базе данных. Пожалуйста, сообщите о проблеме: <code>{{.Hash}}</code>",

	SPAMMYMSG:           "{{if .Spammy}}Теперь эта группа будет спамиться. {{else}}Больше спамить не буду.{{end}}",
	COINFLIPSENABLEDMSG: "Подбросы монетки {{if .Enabled}}разрешены{{else}}запрещены{{end}} в этой группе.",
	LANGUAGEMSG:         "Установлен язык этого чата <code>{{.Language}}</code>.",
	FREEJOIN:            "К этой группе теперь можно присоединиться свободно.",
	EXPENSIVEMSG:          "Каждое сообщение{{with .Pattern}}, содержащее паттерн <code>{{.}}</code>{{end}} будет стоить {{.Price}} сат.",
	EXPENSIVENOTIFICATION: "Сообщение {{.Link}} только что {{if .Sender}}стоило{{else}}заработало{{end}} вам {{.Price}} сат.",
	FREETALK:              "Сообщения снова бесплатны",

	APPBALANCE: `#{{.App | lower}} Баланс: <i>{{printf "%.15g" .Balance}} сат</i>`,

	HELPINTRO: `
<pre>{{.Help}}</pre>
Для более подробной информации по каждой команде пожалуйста введите <code>/help &lt;command&gt;</code>.
    `,
	HELPSIMILAR: "/{{.Method}} команда не найдена. Вы имели в виду /{{index .Similar 0}}?{{if gt (len .Similar) 1}} Или может быть /{{index .Similar 1}}?{{if gt (len .Similar) 2}} Вероятно, {{index .Similar 2}}?{{end}}{{end}}",
	HELPMETHOD: `
<pre>/{{.MainName}} {{.Argstr}}</pre>
{{.Help}}
{{if .HasInline}}
<b>Инлайн запрос</b>
Команда может быть вызвана как <a href="https://core.telegram.org/bots/inline">инлайн запрос</a> из группы или в персональном чате, где бот ещё не добавлен. Синтаксис команды проще: <code>@{{.ServiceId}} {{.InlineExample}}</code>, пользователь должен подождать результат "поиска" команды.{{end}}
{{if .Aliases}}
<b>Алиасы:</b> <code>{{.Aliases}}</code>{{end}}
    `,

	// the "any" is here only for illustrative purposes. if you call this with 'any' it will
	// actually be assigned to the <сатoshis> variable, and that's how the code handles it.
	RECEIVEHELP: `Создаёт BOLT11 счёт с заданным значением сатоши. Полученные токены будут добавлены к вашему балансу в боте. Если вы не укажете количество, это будет счёт с открытым полем значения, в который может быть добавлено любое количество.",

<code>/receive_320_for_something</code> создаёт запрос на 320 сат с описанием "for something"
    `,

	PAYHELP: `Расшифровывает BOLT11 счёт и спрашивает хотите ли вы его оплатить (иначе используйте /paynow). Будет получен такой же результат как если бы пользователь скопировал и вставил счёт в чат с ботом. Фото с изображением QR с зашифрованным счётом также работает (если картинка достаточно качественная).

Просто вставляя <code>lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> расшифровывает и печатает заданный счёт.  

<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> оплачивает счёт без подтверждения.

/withdraw_lnurl_3000 создаёт lnurl и QR код для вывода 3000 сатоши из <a href="https://lightning-wallet.com">совместимого кошелька</a> без запроса подтверждения.
    `,

	SENDHELP: `Отправляет сатоши другим пользователям Telegram. Получатель оповещается в его чате с ботом. Если получатель ещё не запустил бот, или заблокировал его, он не может быть оповещён. В этом случае вы можете отменить транзакцию после, вызвав /transactions.

<code>/tip 100</code>, Если эта команда вызывается в ответ на сообщение в групповом чате, где бот добавлен, то автору сообщения будет начислено 100 сатоши.
<code>/send 500 @username</code> отправляет 500 сатоши пользователю Telegram @username.
<code>/send anonymously 1000 @someone</code> то же, что выше, Telegram пользователь @someone увидит только: "Кто-то отправил вам 1000 сатоши".
    `,

	TRANSACTIONSHELP: `
Показывает все ваши транзакции постранично. Каждая транзакция имеет ссылку, на которую можно нажать, чтобы просмотреть её более подробно.

/transactions показывает все транзакции, начиная с недавних.
<code>/transactions --in</code> показывает только входящие транзакции.
<code>/transactions --out</code> показывает только исходящие транзакции.
    `,

	BALANCEHELP: "Покажет вам текущий баланс в сатоши, а также сумму всего, что вы получили и отправили внутри бота и сумму всех комиссий.",

	FINEHELP: "Выводит пользователю в группе счёт на оплату. Если счёт не оплачивается в течение 15 минут, пользователь исключается и попадает в чёрный список на 1 день.",
	FINEMESSAGE: `⚠️ {{.FinedUser}}, вы были <b>оштрафованы</b> на <i>{{.Sats}} сат</i>{{if .Reason}} по причине <i>{{ .Reason }}</i>{{end}}.

У вас 15 минут на оплату, или вы будете исключены.
    `,
	FINEFAILURE: "{{.User}} не оплатил штраф, был исключён и помещён в чёрный список на 1 день.",
	FINESUCCESS: "{{.User}} оплатил штраф.",
	
	GIVEAWAYHELP: `Создаст кнопку в групповом чате. Первый, кто нажмёт на неё, получит сатоши.

/giveaway_1000: как только кто-либо нажмёт 'Получить' 1000 сатоши будут переведены кликеру.
    `,
	SATSGIVENPUBLIC: "{{.Sats}} сат подарены от {{.From}} пользователю {{.To}}.{{if .ClaimerHasNoChat}} Для управления своими сатоши, начните диалог с @lntxbot.{{end}}",
	CLAIMFAILED:     "Ошибка запроса {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:   "Получить",
	GIVEAWAYMSG:     "{{.User}} {{if .Away}}раздаёт{{else if .Receiver}}@{{.Receiver}}{{else}}тебе{{end}} {{.Sats}} сат!",

	COINFLIPHELP: `Запускает честную лотерею подбрасывания монетки с указанным количеством участников. Все платят одинаковую стоимость входа. Победитель получает всё. Токены перемещаются только в тот момент, когда запускается лотерея.

/coinflip_100_5: 5 участников необходимы, победитель получит 500 сатоши (включая его 100, поэтому чистый выигрыш 400 сатоши).
    `,
	COINFLIPWINNERMSG:      "Вы победитель в подбросе монетки с призом {{.TotalSats}} сат. Проигравшие: {{.Senders}}.",
	COINFLIPGIVERMSG:       "Вы проиграли {{.IndividualSats}} в подбросе монетки. Победителем стал {{.Receiver}}.",
	COINFLIPAD:             "Заплатите {{.Sats}} сат и получите шанс выиграть {{.Prize}}! Осталось {{.SpotsLeft}} из {{.MaxPlayers}} свободных мест!",
	COINFLIPJOIN:           "Играть в лотерею!",
	CALLBACKCOINFLIPWINNER: "Победитель: {{.Winner}}",

	GIVEFLIPHELP: `Начинает случайную раздачу. Вместо подарка первому кликеру, количество будет разыграно между первыми x кликерами.

/giveflip_100_5: 5 участников необходимо 500 сатоши получит победитель от инициатора команды.
    `,
	GIVEFLIPMSG:       "{{.User}} раздаёт {{.Sats}} сат счастливчику из {{.Participants}} участников!",
	GIVEFLIPAD:        "{{.Sats}} будут разданы. Присоединись и получи шанс выиграть! Осталось {{.SpotsLeft}} из {{.MaxPlayers}} мест!",
	GIVEFLIPJOIN:      "Попробую выиграть!",
	GIVEFLIPWINNERMSG: "{{.Sender}} отправил(а) {{.Sats}} сат {{.Receiver}}. Ничего не получили пользователи: {{.Losers}}.{{if .ReceiverHasNoChat}} Для управления своими деньгами начните диалог с @lntxbot.{{end}}",

	FUNDRAISEHELP: `Начинает краудфандинг с заранее определённым количеством участников и вкладом каждого. Если количество участников будет достигнуто, фандрайзинг будет актуализирован. Иначе он будет отменён через несколько часов.

<code>/fundraise 10000 8 @user</code>: Пользователь @user получит 80000 сатоши, если 8 человек присоединятся к компании.
    `,
	FUNDRAISEAD: `
Фандрайзинг {{.Fund}} в пользу {{.ToUser}}!
Необходимо участников: {{.Participants}}
Вклад каждого: {{.Sats}} сат
Присоединились: {{.Registered}}
    `,
	FUNDRAISEJOIN:        "Вкладываюсь!",
	FUNDRAISECOMPLETE:    "Фандрайзинг для {{.Receiver}} завершён!",
	FUNDRAISERECEIVERMSG: "Вы получили {{.TotalSats}} сат от пользователей {{.Senders}}",
	FUNDRAISEGIVERMSG:    "Вы пожертвовали {{.IndividualSats}} в пользу {{.Receiver}}.",

	LIGHTNINGATMHELP: `Возвращает логин и пароль в формате <a href="https://docs.lightningatm.me">LightningATM</a>, разработанного @Z1isenough.

Для конкретных инструкций по интеграции @lntxbot посетите <a href="https://docs.lightningatm.me/lightningatm-setup/wallet-setup/lntxbot">the lntxbot setup tutorial</a> (также есть <a href="https://docs.lightningatm.me/faq-and-common-problems/wallet-communication#talking-to-an-api-in-practice">более детальное техническое описание (на англ.)</a>).
  `,
	BLUEWALLETHELP: `Показывает ваши регистрационные данные для импорта кошелька бота в BlueWallet. Вы можете использовать тот же аккаунт из обоих мест попеременно.

/bluewallet Печатает строчку вроде "lndhub://&lt;login&gt;:&lt;password&gt;@&lt;url&gt;", которая должна быть скопирована и вставлена в BlueWallet функцию импорта.
/bluewallet_refresh очищает предыдущий пароль и печатает новую строку. Вы должны ре-импортировать регистрационные данные в кошелёк BlueWallet после этого шага. Делайте это только в том случае, если предыдущие данные были скомпрометированы.
    `,
	APIPASSWORDUPDATEERROR: "Ошибка обновления пароля. Сообщите о ней: {{.Err}}",
	APICREDENTIALS: `
Это токены для <i>Basic Auth</i>. API совместимо с lndhub.io с добавленными методами.

Полный доступ: <code>{{.Full}}</code>
Выписка счетов: <code>{{.Invoice}}</code>
Доступ на чтение: <code>{{.ReadOnly}}</code>
API Base URL: <code>{{.ServiceURL}}/</code>

/api_full, /api_invoice и /api_readonly будут показывать эти специфические токены вместе с QR кодами.
/api_url покажет QR код для API Base URL.

Сохраняйте эти токены в секрете. Если они будут скомпрометированы, вызывайте команду /api_refresh для их полной замены.
    `,

	HIDEHELP: `Скрывает сообщение, которое может быть открыто позже после оплаты.
<code>/hide 500 'превью сообщения'</code> отошлите это в ответ на любое сообщение, с видео, аудио, изображением или текстом, и оно будет скрыто на пейволлом в 500 сатоши.

Модификаторы:
  <code>--crowdfund &lt;number&gt;</code> включает краудфандинг скрытого сообщения.
  <code>--private</code> открывает сообщение в приватном чате того, кто оплатил его.
  <code>--revealers &lt;number&gt;</code> позволяет только первым <code>&lt;number&gt;</code> участникам увидеть сообщение, затем оно становится недоступным.
    `,

	REVEALHELP: `Открывает сообщение, которое было скрыто. Автор скрытого сообщения не будет раскрыт. Как только сообщение скрыто, оно может быть открыто глобально, но только теми, кто знает скрытый id.

Приглашение к открытию может быть создано в чате или группе нажатием кнопки "Шэрить" после того, как было скрыто сообщение. Затем применяются обычные правила открытия сообщения, смотрите /help_hide для подробной справки.

<code>/reveal 5c0b2rh4x</code> создаёт приглашение открыть скрытое сообщение 5c0b2rh4x, если оно существует.
    `,
	HIDDENREVEALBUTTON:   `{{.Sats}} сат открыть {{if .Public}}тут же{{else}}приватно{{end}}. {{if gt .Crowdfund 1}}Краудфандинг: {{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Допущены публикаторы: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "Здесь скрыто сообщение. {{.Sats}} сат нужно, чтобы его открыть.",
	HIDDENWITHID:         "Сообщение скрыто с id <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Будет раскрыто публично один раз {{.Message.Crowdfund}} люди заплатят {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Будет раскрыто приватно {{.Message.Times}} пользователям {{else if .Message.Public}}Будет раскрыто публично как только один человек заплатит {{.Message.Satoshis}}{{else}}Будет раскрыто приватно любому заплатившему {{end}}.",
	HIDDENSOURCEMSG:      "Скрытое сообщение <code>{{.Id}}</code> было открыто {{.Revealers}}. Вы получили {{.Sats}} сат.",
	HIDDENREVEALMSG:      "{{.Sats}} сат заплачено для открытия сообщения <code>{{.Id}}</code>.",
	HIDDENMSGNOTFOUND:    "Скрытое сообщение не найдено.",
	HIDDENSHAREBTN:       "Поделиться в другом чате",

	TOGGLEHELP: `Включает/выключает функции бота в группах. В супергруппах команда может быть запущена только администраторами.

/toggle_ticket_10  начинает брать комиссию для всех новых пользователей. Полезная функция антиспама. Деньги идут владельцу группы.
/toggle_ticket перестаёт брать комиссию с новых участников.
/toggle_language_ru меняет язык чата на Русский, 
/toggle_spammy 'спамная' функция выключена по умолчанию. Когда она включена, нотификации о tip командах будут отсылаться в чат, вместо того, чтобы обрабатываться приватно.
`,

	SATS4ADSHELP: `
Sats4ads это маркетплейс рекламы в Telegram. Платите сатоши, чтобы показывать рекламу остальным, получайте сатоши за каждое рекламное объявление, которое вы видите.

Цены для каждого пользователя устанавливаются в миллисатоши за символ.
Каждое объявление также включает фиксированную плату в 1 сат.
Картинки и видео эквивалентны 100 символам.

Для отправки рекламы вам нужно отправить рекламное сообщение боту, затем ответить на это рекламное сообщение с использованием <code>/sats4ads broadcast ...</code> как это описано ниже. Вы можете добавить <code>--max-rate=500</code> и <code>--skip=0</code> для лучшего контроля над вещанием сообщения. Это настройки по умолчанию.

/sats4ads_on_15 устанавливает аккаунт в режим просмотра рекламы. Любой может опубликовать сообщение для вас за 15 миллисатоши за символ. Вы можете изменить эту цену
/sats4ads_off выключает подписку на рекламные сообщения.
/sats4ads_rates показывает разбиение пользователей по ценовым группам. Полезно, чтобы планировать бюджет.
/sats4ads_broadcast_1000 вещает сообщение. Последняя цифра равна максимуму сатоши, которые будут потрачены. Более дешёвые подписчики рекламы предпочтительны. Эта команда должна вызываться в ответ на другое сообщение, содержание которого будет использовано в качестве текста рекламы.
    `,
	SATS4ADSTOGGLE:    `#sats4ads {{if .On}}Смотреть рекламу и получать {{printf "%.15g" .Sats}} сат за символ.{{else}}Вы больше не увидите рекламы.{{end}}`,
	SATS4ADSBROADCAST: `#sats4ads {{if .NSent}}Сообщение отправлено {{.NSent}} раз с полной стоимостью {{.Sats}} сат ({{dollar .Sats}}).{{else}}Не могу найти подписчиков с подходящими параметрами. /sats4ads_rates{{end}}`,
	SATS4ADSSTART:     `Сообщение в рассылке.`,
	SATS4ADSPRICETABLE: `#sats4ads Количество пользователей в каждом диапазоне цены.
	
{{range .Rates}}<code>{{.UpToRate}} мсат</code>: <i>пользователей всего {{.NUsers}}</i>
{{else}}
<i>Никто ещё не зарегистрировался на просмотры объявлений.</i>
{{end}}
Каждое сообщение стоит такую цену <i>за символ</i> + <code>1 сат</code> за каждого пользователя.
    `,
	SATS4ADSADFOOTER: `[#sats4ads: {{printf "%.15g" .Sats}} сат]`,
	SATS4ADSVIEWED:   `Просмотрено`,

	HELPHELP: "Показывает полную справку или справку о конкретной команде.",

	STOPHELP: "Бот перестаёт отсылать оповещения.",

	PAYPROMPT: `
{{if .Sats}}<i>{{.Sats}} сат</i> ({{dollar .Sats}})
{{end}}{{if .Description}}<i>{{.Description}}</i>{{else}}<code>{{.DescriptionHash}}</code>{{end}}
{{if .ReceiverName}}
<b>Получатель</b>: {{.ReceiverName}}{{end}}
<b>Хэш</b>: <code>{{.Hash}}</code>{{if ne .Currency "bc"}}
<b>Цепь</b>: {{.Currency}}{{end}}
<b>Создано</b>: {{.Created}}
<b>Истекает</b>: {{.Expiry}}{{if .Expired}} <b>[ИСТЁК]</b>{{end}}{{if .Hints}}
<b>Подсказки</b>: {{range .Hints}}
- {{range .}}{{.ShortChannelId | channelLink}}: {{.PubKey | nodeAliasLink}}{{end}}{{end}}{{end}}
<b>Получатель</b>: {{.Payee | nodeLink}} (<u>{{.Payee | nodeAlias}}</u>)

{{if .Sats}}Заплатить счёт выше?{{if .IsDiscord}}
React with a :zap: to confirm.{{end}}
{{else}}<b>Ответьте с желаемым количеством для подтверждения</b>
{{end}}
    `,
	FAILEDDECODE: "Ошибка декодирования счёта: {{.Err}}",
	BALANCEMSG: `🏛
<b>Полный баланс</b>: {{printf "%.15g" .Sats}} сат ({{dollar .Sats}})
<b>Доступный баланс</b>: {{printf "%.15g" .Sats}} сат ({{dollar .Usable}})
<b>Всего получено</b>: {{printf "%.15g" .Received}} сат
<b>Всего отправлено</b>: {{printf "%.15g" .Sent}} сат
<b>Всего комиссий оплачено</b>: {{printf "%.15g" .Fees}} сат

/transactions
#balance
    `,
	TAGGEDBALANCEMSG: `
<b>Всего разница </b> <code>получено - потрачено</code> <b>на внутренние и внешние</b> /apps<b>:</b>

{{range .Balances}}<code>{{.Tag}}</code>: <i>{{printf "%.15g" .Balance}} сат</i>  ({{dollar .Balance}})
{{else}}
<i>Пока не совершено транзакций данного типа.</i>
{{end}}
#balance
    `,
	FAILEDUSER: "Не могу распознать получателя.",
	LOTTERYMSG: `
Раунд лотереи стартовал!
Билет на вход: {{.EntrySats}} сат
Всего участников: {{.Participants}}
Приз: {{.Prize}}
Зарегистрировано: {{.Registered}}
    `,
	INVALIDPARTNUMBER: "Неверное количество участников: {{.Number}}",
	USERSENTTOUSER:    "💛 {{menuItem .Sats .RawSats true }} ({{dollar .Sats}}) отправлено {{.User}}{{if .ReceiverHasNoChat}} (не могу уведомить {{.User}} так как он не начал диалог с ботом{{end}}",
	USERSENTYOUSATS:   "💛 {{.User}} отправил вам {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}){{if .BotOp}} в ходе {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:  "💛 Кто-то отослал вам {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}).",
	FAILEDSEND:        "Ошибка отправки: ",
	QRCODEFAIL:        "QR код не был прочитан: {{.Err}}",
	SAVERECEIVERFAIL:  "Ошибка сохранения получателя. Это вероятно баг.",
	MISSINGRECEIVER:   "Не хватает получателя!",
	GIVERCANTJOIN:     "Даритель не может присоединиться!",
	CANTJOINTWICE:     "Нельзя участвовать снова!",
	CANTREVEALOWN:     "Не можете раскрыть собственное скрытое послание!",
	CANTCANCEL:        "У вас нет возможности отменить это.",
	FAILEDINVOICE:     "Ошибка генерации счёта: {{.Err}}",
	STOPNOTIFY:        "Оповещения выключены.",
	START: `
⚡️ @lntxbot, a <b>Bitcoin</b> Lightning wошелёк в вашем Телеграме.

🕹️  <b>Основные команды</b>
<b>&lt;invoice&gt;</b> - Просто вставьте счет или LNURL, чтобы расшифровать или оплатить его.
<b>/balance</b> - Показывает ваш баланс.
<b>/tip &lt;amount;&gt;</b> - Отправьте это в ответ на другое сообщение в группе, чтобы оставить чаевые.
<b>/invoice &lt;amount&gt; &lt;description&gt;</b> - Генерирует запрос платежа Lightning: <code>/invoice 400 'split coffee'</code>.
<b>/send &lt;amount;&gt; &lt;user&gt;</b> - Отправляет несколько сатоши другому пользователю: <code>/send 100 @fiatjaf</code>

🍎 <b>Другие вещи, которые вы можете сделать</b>
- Используйте <b>/send</b> для отправки денег на любой <a href="https://lightningaddress.com">Lightning-адрес</a>.
- Получайте деньги на свой адрес {{ .YourName }}@lntxbot.com или at https://lntxbot.com/@{{ .YourName }}.
- Считайте <code>4*usd</code> или <code>eur*rand()</code> каждый раз, когда надо указать количество в сатоши.
- Используйте <b>/withdraw lnurl &lt;amount&gt;</b> для ваучера LNURL-withdraw.

🎮 <b>Веселые или полезные команды</b>
<b>/sats4ads</b> Получайте деньги за просмотр сообщений, вы контролируете их количество - или отправляйте рекламу всем подряд. Большие коэффициенты конверсии!
<b>/giveaway</b> и <b>/giveflip</b> - Дарите деньги в чатах.
<b>/hide</b> - Скрыть сообщение. Люди должны заплатить, чтобы увидеть его. Множество способов раскрытия: публичный, частный, краудфандинг. Поддерживаются мультимедиа.
<b>/coinflip &lt;amount&gt; &lt;number_of_participants&gt;</b> - Создает лотерею, в которой может принять участие любой желающий <i>(10 сат комиссия lntxbot)</i>.

🐟 <b>Инлайн команды</b> - <i>могут быть использованы в любом чате, даже если бот не присутствует в нем</i>
<code>@lntxbot give &lt;amount&gt;</code> - Создает кнопку в приватном чате для передачи денег другой стороне.
<code>@lntxbot coinflip/giveflip/giveaway</code> - То же самое, что и версия слэш-команды, но может использоваться в группах без @lntxbot.
<code>@lntxbot invoice &lt;amount&gt;</code> - Выставляет счет и отправляет его в чат.

🏖  <b>Продвинутые команды</b>
<b>/bluewallet</b> - Подключить BlueWallet или Zeus к вашему аккаунту @lntxbot.
<b>/transactions</b> - Перечисляет все ваши транзакции, постранично.
<b>/help &lt;command;&gt;</b> - Показывает подробную справку по определенной команде.
<b>/paynow &lt;invoice&gt;</b> - Оплатить счет без запроса.
<b>/send --anonymous &lt;amount&gt; &lt;user&gt;</b> - Отправить анонимно. Получатель не знает, кто отправил ему саты.

🏛  <b>Управление группами</b>
<b>/toggle ticket &lt;amount&gt;</b> - Выставить цену в сатоши за вступление в вашу группу. Отличный антиспам! Деньги идут владельцу группы.
<b>/toggle renamable &lt;amount&gt;</b> - Позволяет людям использовать /rename для переименования вашей группы, и вы получаете деньги.
<b>/toggle expensive &lt;amount&gt; &lt;regex pattern&gt;</b> - Взимать плату с людей за неправильные слова в вашей группе (или оставить пустым, чтобы взимать плату за все сообщения).
<b>/fine &lt;amount&gt;</b> - Заставить людей платить штраф, чтобы остаться в группе.

---

Существуют и другие команды, но их изучение остается на усмотрение пользователя.

Удачи! 🍽️
	`,
	WRONGCOMMAND:    "Не могу понять команду. /help",
	RETRACTQUESTION: "Вернуть не затребованное поощрение?",
	RECHECKPENDING:  "Перепроверить платёж в обработке?",

	TXNOTFOUND: "Не могу найти транзакцию {{.HashFirstChars}}.",
	TXINFO: `{{.Txn.Icon}} <code>{{.Txn.Status}}</code> {{.Txn.PeerActionDescription}} на {{.Txn.Time | time}} {{if .Txn.IsUnclaimed}}<b>[💤 ВОСТРЕБОВАНА]</b>{{end}}
<i>{{.Txn.Description}}</i>{{if not .Txn.TelegramPeer.Valid}}
{{if .Txn.Payee.Valid}}<b>Оплатил</b>: {{.Txn.Payee.String | nodeLink}} (<u>{{.Txn.Payee.String | nodeAlias}}</u>){{end}}
<b>Хэш</b>: <code>{{.Txn.Hash}}</code>{{end}}{{if .Txn.Preimage.String}}
<b>Секрет (Preimage)</b>: <code>{{.Txn.Preimage.String}}</code>{{end}}
<b>Количество</b>: <i>{{.Txn.Amount | printf "%.15g"}} сат</i> ({{dollar .Txn.Amount}})
{{if not (eq .Txn.Status "RECEIVED")}}<b>Комиссия</b>: <i>{{printf "%.15g" .Txn.Fees}}</i>{{end}}
{{.LogInfo}}
    `,
	TXLIST: `<b>{{if .Offset}}Транзакция от {{.From}} к {{.To}}{{else}}Последние {{.Limit}} транзакций{{end}}</b>
{{range .Transactions}}<code>{{.StatusSmall}}</code> <code>{{.Amount | paddedSatoshis}}</code> {{.Icon}} {{.PeerActionDescription}}{{if not .TelegramPeer.Valid}}<i>{{.Description}}</i>{{end}} <i>{{.Time | timeSmall}}</i> /tx_{{.HashReduced}}
{{else}}
<i>Ещё нет ни одной транзакции</i>
{{end}}
    `,
	TXLOG: `<b>Попытки маршрутов</b>{{if .PaymentHash}} (<code>{{.PaymentHash}}</code>){{end}}:
{{range $t, $try := .Tries}}{{if $try.Success}}✅{{else}}❌{{end}} {{range $h, $hop := $try.Route}}➠{{.Channel | channelLink}}{{end}}{{with $try.Error}}{{if $try.Route}}
{{else}} {{end}}<i>{{. | makeLinks}}</i>
{{end}}{{end}}
    `,
}
