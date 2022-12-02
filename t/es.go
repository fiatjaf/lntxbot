package t

var ES = map[Key]string{
	NO:          "No",
	YES:         "Sí",
	CANCEL:      "Cancelar",
	CANCELED:    "Cancelado.",
	COMPLETED:   "¡Completado!",
	CONFIRM:     "Confirmar",
	PAYAMOUNT:   `Pagar {{.Sats | printf "%.15g"}}`,
	FAILURE:     "Fallo.",
	PROCESSING:  "Procesando...",
	WITHDRAW:    "¿Retirar?",
	ERROR:       "🔴{{if .App}}#{{.App | lower}} {{end}}Error{{if .Err}}: {{.Err}}{{else}}!{{end}}",
	CHECKING:    "Comprobando...",
	TXPENDING:   "El pago aún está en curso. Por favor, intente de nuevo más tarde.",
	TXCANCELED:  "Transacción cancelada.",
	UNEXPECTED:  "Error inesperado: por favor, repórtelo.",
	MUSTBEADMIN: "Este comando debe ser llamado por un administrador.",
	MUSTBEGROUP: "Este comando debe ser utilizado en un grupo.",

	CALLBACKWINNER:  "Ganador: {{.Winner}}",
	CALLBACKERROR:   "{{.BotOp}} error{{if .Err}}: {{.Err}}{{else}}.{{end}}",
	CALLBACKEXPIRED: "{{.BotOp}} expiró.",
	CALLBACKATTEMPT: "Intentando el pago. /tx_{{.Hash}}",
	CALLBACKSENDING: "Enviando el pago.",

	INLINEINVOICERESULT:  "Solicitud de pago por {{.Sats}} sat.",
	INLINEGIVEAWAYRESULT: "Regalar {{.Sats}} sat {{if .Receiver}}a @{{.Receiver}}{{else}}{{end}}",
	INLINEGIVEFLIPRESULT: "Regalar {{.Sats}} sat a uno entre {{.MaxPlayers}} participantes",
	INLINECOINFLIPRESULT: "Lotería con tasa de entrada de {{.Sats}} sat para {{.MaxPlayers}} participantes",
	INLINEHIDDENRESULT:   "{{.HiddenId}} ({{if gt .Message.Crowdfund 1}}crowd:{{.Message.Crowdfund}}{{else if gt .Message.Times 0}}priv:{{.Message.Times}}{{else if .Message.Public}}pub{{else}}priv{{end}}): {{.Message.Content}}",

	LNURLUNSUPPORTED: "Ese tipo de lnurl no se admite aquí.",
	LNURLERROR:       `<b>{{.Host}}</b> Error de lnurl: {{.Reason}}`,
	LNURLAUTHSUCCESS: `
 ¡Éxito de lnurl-auth!
 
 <b>Dominio</b>: <i>{{.Host}}</i>
<b>Llave pública</b>: <i>{{.PublicKey}}</i>
`,
	LNURLPAYPROMPT: `🟢 <code>{{.Domain}}</code> espera que {{if .FixedAmount}}<i>{{.FixedAmount | printf "%.15g"}} sat</i>{{else}}un valor entre <i>{{.Min | printf "%.15g"}}</i> y <i>{{.Max | printf "%.15g"}} sat</i>{{end}} para:
 
 <code>{{if .Long}}{{.Long | html}}{{else}}{{.Text | html}}{{end}}</code>{{if .WillSendPayerData}}
 
 ---
 
 -Su nombre y/o claves de autenticidad serán enviados al beneficiario.
 -Para evitarlo, use <code>/lnurl --anonymous &lt;lnurl&gt;</code>.
 {{end}}
 
 {{if not .FixedAmount}}<b>Responda con el monto (en satoshis, entre <i>{{.Min | printf "%.15g"}}</i> y <i>{{.Max | printf "%.15g"}}</i>) para confirmar.</b>{{end}}
  `,
	LNURLPAYPROMPTCOMMENT: `📨 <code>{{.Domain}}</code> espera un comentario.
        
 <b>Para confirmar el pago, responde con algo de texto</b>`,
	LNURLPAYAMOUNTSNOTICE: `<code>{{.Domain}}</code> espera que {{if .Exact}}{{.Min | printf "%.3f"}}{{else if .NoMax}}al menos{{.Min | printf "%.0f"}}{{else}}entre {{.Min | printf "%.0f"}} y {{.Max | printf "%.0f"}}{{end}} sat.`,
	LNURLPAYSUCCESS: `<code>{{.Domain}}</code> dice:
{{.Text}}
{{if .DecipherError}}No se descifró ({{.DecipherError}}):
{{end}}{{if .Value}}<pre>{{.Value}}</pre>
{{end}}{{if .URL}}<a href="{{.URL}}">{{.URL}}</a>{{end}}
    `,
	LNURLPAYMETADATA: `#lnurlpay metadata:
<b>dominino</b>: <i>{{.Domain}}</i>
<b>transacció</b>: /tx_{{.HashFirstChars}}
    `,
	LNURLBALANCECHECKCANCELED: "Las comprobaciones de saldo automáticas de {{.Service}} has sido canceladas.",

	TICKETSET:         "Los nuevos participantes tendrán que pagar una factura de {{.Sat}} sat (asegúrate de haber puesto a @lntxbot como administrador para que esto funcione).",
	TICKETUSERALLOWED: "Ticket pagado. {{.User}} permitido.",
	TICKETMESSAGE: `⚠️ {{.User}}, este grupo requiere que usted pague {{.Sats}} sat para poder unirse.
        
Tienes 15 minutos para hacerlo o serás expulsado y baneado por un día.
    `,

	RENAMABLEMSG:      "Cualquiera puede cambiar el nombre de este grupo siempre que paguen {{.Sat}} sat (asegúrate de que has puesto a @lntxbot como administrador para que esto funcione).",
	RENAMEPROMPT:      "Pagar <b>{{.Sats}} sat</b> para cambiar el nombre de este grupo por <i>{{.Name}}</i>?",
	GROUPNOTRENAMABLE: "¡Este grupo no se puede renombrar!",

	INTERNALPAYMENTUNEXPECTED: "Ha ocurrido algo extraño. Si se trata de una factura interna, fallará. Puede que la factura haya caducado o algo más que desconocemos. Si se trata de una factura externa, ignora esta advertencia.",
	PAYMENTFAILED:             "❌ Pago <code>{{.Hash}}</code> fallido.\n\n<i>{{.FailureString}}</i>",
	PAIDMESSAGE: `✅ Pagado con <i>{{printf "%.15g" .Sats}} sat</i> ({{dollar .Sats}}){{if .Fee}} (+ <i>{{.Fee}}</i> fee){{end}}.
{{if .Hash}}
<b>Hash:</b> <code>{{.Hash}}</code>{{if .Preimage}}
<b>Prueba:</b> <code>{{.Preimage}}</code>{{end}}

/tx_{{.ShortHash}} ⚡️ #tx{{end}}`,
	OVERQUOTA:           "Has superado tu cuota semanal de {{.App}}.",
	RATELIMIT:           "Esta acción está limitada por tarifas. Por favor, espere 30 minutos.",
	DBERROR:             "Error en la base de datos: falló en marcar la transacción como no pendiente.",
	INSUFFICIENTBALANCE: `Saldo insuficiente para {{.Purpose}}. Necesitas {{.Sats | printf "%.15g"}} sat más.`,

	PAYMENTRECEIVED: `
      ⚡️ Pago recibido{{if .SenderName}} de <i>{{ .SenderName }}</i>{{end}}: {{.Sats}} sat ({{dollar .Sats}}). /tx_{{.Hash}}{{if .Message}} {{.Message | messageLink}}{{end}} #tx
      {{if .Comment}}
📨 <i>{{.Comment}}</i>
      {{end}}
    `,
	FAILEDTOSAVERECEIVED: "Se recibió el pago, pero no se pudo guardar en la base de datos. Por favor, informe este problema: <code>{{.Hash}}</code>",

	SPAMMYMSG:             "{{if .Spammy}}Este grupo es ahora ''spammy'' (embasura).{{else}}No hay más spam.{{end}}",
	COINFLIPSENABLEDMSG:   "Los Coinflips están {{if .Enabled}}habilitadas{{else}}deshabilitadas{{end}} en este grupo.",
	LANGUAGEMSG:           "El idioma de este chat está en <code>{{.Language}}</code>.",
	FREEJOIN:              "Ahora es posible unirse a este grupo de forma gratuita.",
	EXPENSIVEMSG:          "Cada mensaje de este grupo{{with .Pattern}} que contenga el patrón <code>{{.}}</code>{{end}} costará {{.Price}} sat.",
	EXPENSIVENOTIFICATION: "El mensaje {{.Link}}{{if .Sender}}te costó{{else}}te generó{{end}}{{.Price}} sat.",
	FREETALK:              "Los mensajes vuelven a ser gratuitos.",

	APPBALANCE: `#{{.App | lower}} Saldo: <i>{{printf "%.15g" .Balance}} sat</i>`,

	HELPINTRO: `
<pre>{{.Help}}</pre>
Para obtener más información sobre cada comando, escriba <code>/help &lt;command&gt;</code>.
    `,
	HELPSIMILAR: "/{{.Method}} comando no encontrado. ¿Quieres decir /{{index .Similar 0}}?{{if gt (len .Similar) 1}} O quizás /{{index .Similar 1}}?{{if gt (len .Similar) 2}} Tal vez /{{index .Similar 2}}?{{end}}{{end}}",
	HELPMETHOD: `
<pre>/{{.MainName}} {{.Argstr}}</pre>
{{.Help}}
{{if .HasInline}}
<b>Consulta Inline</b>
También se puede llamar como <a href="https://core.telegram.org/bots/inline"> consulta inline</a> de los chats en los que no se ha agregado el bot. La sintaxis es similar, pero simplificada: <code>@{{.ServiceId}} {{.InlineExample}}</code> espera a que aparezca un "resultado de búsqueda".{{end}}
{{if .Aliases}}
<b>Aliases:</b> <code>{{.Aliases}}</code>{{end}}
    `,

	// el 'any' <cualquiera> está aquí sólo con fines ilustrativos. si llamas a esto con 'any'
	// en realidad se asignará a la variable <satoshis>, y así es como lo maneja el código.
	RECEIVEHELP: `Genera una factura BOLT11 con el valor satoshi dado. El importe se añadirá a tu saldo de @lntxbot. Si no proporcionas la cantidad, será una factura abierta que puede ser pagada con cualquier cantidad.",

<code>/receive_320_for_something</code> genera una factura por 320 sat con la descripción 'for something' (para algo)
    `,

	PAYHELP: `Decodifica una factura de BOLT11 y pregunta si quieres pagarla (unless /paynow). Esto es lo mismo que pegar o reenviar una factura directamente en el chat. Tomar una foto del código QR que contiene una factura funciona igual de bien (si la foto es clara).

Solo pega <code>lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> decodifica y solicita el pago de la factura dada.  

<code>/paynow lnbc1u1pwvmypepp5kjydaerr6rawl9zt7t2zzl9q0rf6rkpx7splhjlfnjr869we3gfqdq6gpkxuarcvfhhggr90psk6urvv5cqp2rzjqtqkejjy2c44jrwj08y5ygqtmn8af7vscwnflttzpsgw7tuz9r407zyusgqq44sqqqqqqqqqqqqqqqgqpcxuncdelh5mtthgwmkrum2u5m6n3fcjkw6vdnffzh85hpr4tem3k3u0mq3k5l3hpy32ls2pkqakpkuv5z7yms2jhdestzn8k3hlr437cpajsnqm</code> paga la factura dada sin pedir confirmación.

/withdraw_lnurl_3000 genera un <b>lnurl y un código QR para retirar 3000</b> satoshis de un <a href="https://lightning-wallet.com">monedero compatible</a> sin pedir confirmación.
    `,

	SENDHELP: `Envía satoshis a otros usuarios de Telegram. El receptor recibe una notificación en su chat con @lntxbot. Sin embargo, si el receptor nunca ha hablado con el bot o lo ha bloqueado, no puede ser notificado. En ese caso puede cancelar la transacción después en la vista de /transactions.

<code>/tip 100</code>, cuando se envía como respuesta a un mensaje en un grupo donde el bot está agregado, envía 100 satoshis al autor del mensaje.
<code>/send 500 @username</code> envía 500 satoshis al usuario de Telegram @username &lt;nombre_de_usuario&gt;.
<code>/send anonymously 1000 @someone</code> lo mismo que arriba, pero el usuario de Telegram  @someone verá sólo: "Someone has sent you 1000 satoshis" (Alguien te ha enviado 1000 satoshis).
    `,

	TRANSACTIONSHELP: `
Enumera todas sus transacciones con controles de paginación. Cada transacción tiene un enlace en el que se puede hacer clic para obtener más información.

/transactions enumera todas las transacciones, desde la más reciente.
<code>/transactions --in</code> enumera sólo las transacciones entrantes.
<code>/transactions --out</code> enumera sólo las transacciones salientes.
    `,

	BALANCEHELP: "Muestra tu saldo actual en satoshis, además de la suma de todo lo que has recibido y enviado dentro del bot y el importe total de las tasas pagadas.",

	FINEHELP: "Pide a un usuario de un grupo que pague una tasa. Si no pagan en 15 minutos son expulsados del grupo y baneados durante un día.",
	FINEMESSAGE: `⚠️ {{.FinedUser}}, fuiste <b>multado</b> por <i>{{.Sats}} sat</i>{{if .Reason}} por: <i>{{ .Reason }}</i>{{end}}.
      
Tienes 15 minutos para pagar o serás expulsado.
    `,
	FINEFAILURE: "{{.User}} no pagó la multa y fue expulsado y baneado por un día.",
	FINESUCCESS: "{{.User}} ha pagado la multa.",

	GIVEAWAYHELP: `Crea un botón en un chat de grupo. La primera persona que haga clic en el botón se lleva los satoshis.
      
/giveaway_1000: una vez que alguien haga clic en el botón "Claim" (reclamar), se le transferirán 1000 satoshis.
    `,
	SATSGIVENPUBLIC: "{{.Sats}} sat dados de {{.From}} para {{.To}}.{{if .ClaimerHasNoChat}} Para gestionar sus fondos, inicie una conversación con @lntxbot.{{end}}",
	CLAIMFAILED:     "No se ha podido reclamar {{.BotOp}}: {{.Err}}",
	GIVEAWAYCLAIM:   "Reclamar",
	GIVEAWAYMSG:     "{{.User}} te está dando {{if .Away}}{{else if .Receiver}}@{{.Receiver}}{{else}}{{end}} {{.Sats}} sats!",

	COINFLIPHELP: `Inicia una lotería justa con el número de participantes dado. Todos pagan la misma tasa de inscripción. El ganador se lo lleva todo. Los fondos sólo se mueven de las cuentas de los participantes cuando la lotería se hace efectiva.

/coinflip_100_5: Se necesitan 5 participantes, el ganador se llevará 500 satoshis (incluyendo sus propios 100, por lo que son 400 satoshis netos).
    `,
	COINFLIPWINNERMSG:      "Eres el ganador de una lotería para un premio de {{.TotalSats}} sat. Los perdedores fueron: {{.Senders}}.",
	COINFLIPGIVERMSG:       "Has perdido {{.IndividualSats}} en una lotería. El ganador fue {{.Receiver}}.",
	COINFLIPAD:             "Paga {{.Sats}} y tenga la oportunidad de ganar {{.Prize}}! ¡Quedan {{.SpotsLeft}} de {{.MaxPlayers}} puesto{{s .SpotsLeft}}!",
	COINFLIPJOIN:           "¡Únete a la lotería!",
	CALLBACKCOINFLIPWINNER: "Ganador de la lotería: {{.Winner}}",

	GIVEFLIPHELP: `Inicia una lotería, pero en lugar de dar a la primera persona que haga clic, la cantidad se sortea entre los primeros x clics.

/giveflip_100_5: Se necesitan 5 participantes, el ganador recibirá 100 satoshis del emisor del comando.
    `,
	GIVEFLIPMSG:       "{{.User}} está dando {{.Sats}} sat a una persona afortunada de {{.Participants}}!",
	GIVEFLIPAD:        "{{.Sats}} a regalar. ¡Únase y tenga la oportunidad de ganar! ¡Quedan {{.SpotsLeft}} de {{.MaxPlayers}} puesto{{s .SpotsLeft}}!",
	GIVEFLIPJOIN:      "¡Intenta ganar!",
	GIVEFLIPWINNERMSG: "{{.Sender}} envió {{.Sats}} a {{.Receiver}}. Estos no consiguieron nada: {{.Losers}}.{{if .ReceiverHasNoChat}} Para gestionar sus fondos, inicie una conversación con @lntxbot.{{end}}",

	FUNDRAISEHELP: `Inicia un evento de crowdfunding con un número predefinido de participantes y una cantidad de contribución. Si el número de participantes contribuye, se actualizará. En caso contrario, se cancelará en unas horas.

<code>/fundraise 10000 8 @user</code>: El @user de Telegram recibirá 80000 satoshis después de que 8 personas contribuyan.
    `,
	FUNDRAISEAD: `
Recolecta de {{.Fund}} para {{.ToUser}}!
Colaboradores necesarios para completarla: {{.Participants}}
Cada uno paga: {{.Sats}} sat
Han contribuido: {{.Registered}}
    `,
	FUNDRAISEJOIN:        "¡Contribuye!",
	FUNDRAISECOMPLETE:    "Recolecta para {{.Receiver}} completada!",
	FUNDRAISERECEIVERMSG: "Has recibido {{.TotalSats}} sat de una recolecta de {{.Senders}}",
	FUNDRAISEGIVERMSG:    "Has dado {{.IndividualSats}} en una recolecta para {{.Receiver}}.",

	LIGHTNINGATMHELP: `Te da las credenciales en el formato especificado por @Z1isenough para <a href="https://docs.lightningatm.me">LightningATM</a>.

Para obtener documentación específica sobre cómo configurarlo con @lntxbot, visite <a href="https://docs.lightningatm.me/lightningatm-setup/wallet-setup/lntxbot">el tutorial de configuración de lntxbot</a> (también hay <a href="https://docs.lightningatm.me/faq-and-common-problems/wallet-communication#talking-to-an-api-in-practice">una información más detallada y técnica</a>).
  `,
	BLUEWALLETHELP: `Te da tus credenciales para importar tu monedero bot en BlueWallet. Puedes usar la misma cuenta de ambos sitios indistintamente.

/bluewallet imprime una secuencia como "lndhub://&lt;login&gt;:&lt;password&gt;@&lt;url&gt;" que debe copiarse y pegarse en la pantalla de importación de BlueWallet.
/bluewallet_refresh borra su contraseña anterior e imprime una nueva cadena. Tendrás que volver a importar las credenciales en BlueWallet después de este paso. Hazlo sólo si tus credenciales anteriores fueron comprometidas/hackeadas.
    `,
	APIPASSWORDUPDATEERROR: "Error al actualizar la contraseña. Por favor, informe: {{.Err}}",
	APICREDENTIALS: `
Estas son las fichas para <i>Basic Auth</i>. La API es compatible con lndhub.io con algunos métodos adicionales.

Acceso total: <code>{{.Full}}</code>
Acceso a la factura: <code>{{.Invoice}}</code>
Acceso de sólo lectura: <code>{{.ReadOnly}}</code>
API Base URL: <code>{{.ServiceURL}}/</code>

/api_full, /api_invoice y /api_readonly mostrará estos tokens específicos junto con los códigos QR.
/api_url mostrará un código QR para la API Base URL.

Mantén estos tokens en secreto. Si se filtran por alguna razón, ingrese /api_refresh para reemplazar todo.
    `,

	HIDEHELP: `Oculta un mensaje para poder desbloquearlo más tarde con un pago.
<code>/hide 500 'contenido a mostrar'</code>, envíe esto en respuesta a cualquier mensaje, con vídeo, audio, imágenes o texto, y se ocultará tras una tarifa de 500 satoshis.

Modificadores:
  <code>--crowdfund &lt;number&gt;</code> permite el crowdfunding público de mensajes ocultos.
  <code>--privado</code> revela el mensaje oculto en privado al contribuyente en lugar del grupo.
  <code>--reveladores &lt;number&gt;</code> sólo permite a lo primeros <code>&lt;number&gt;</code> participantes ver el mensaje oculto, entonces el aviso expira.
    `,
	REVEALHELP: `Revela un mensaje que estaba previamente oculto. El autor del mensaje oculto nunca se revela. Una vez que un mensaje está oculto, está disponible para ser revelado globalmente, pero sólo por aquellos que conocen su id oculto.

También se puede crear un aviso de revelación en un grupo o chat haciendo clic en el botón "compartir" después de ocultar el mensaje, entonces se aplican las reglas estándar para revelar mensajes, ver /help_hide para más información.

<code>/reveal 5c0b2rh4x</code> crea un aviso para revelar el mensaje oculto 5c0b2rh4x, si es que existe.
    `,
	HIDDENREVEALBUTTON:   `{{.Sats}} sat para revelar {{if .Public}}en el sitio{{else}}en privado{{end}}. {{if gt .Crowdfund 1}}{{.HavePaid}}/{{.Crowdfund}}{{else if gt .Times 0}}Left: {{.HavePaid}}/{{.Times}}{{end}}`,
	HIDDENDEFAULTPREVIEW: "Aquí se esconde un mensaje. {{.Sats}} sat necesarios para desbloquear.",
	HIDDENWITHID: `Mensaje oculto con id <code>{{.HiddenId}}</code>. {{if gt .Message.Crowdfund 1}}Se revelará públicamente una vez {{.Message.Crowdfund}} la gente pague {{.Message.Satoshis}}{{else if gt .Message.Times 0}}Se revelará en privado a los primeros {{.Message.Times}} contribuyentes{{else if .Message.Public}}Se revelará públicamente una vez que una persona pague {{.Message.Satoshis}}{{else}}Se revelará en privado a cualquier contribuyente{{end}}.

{{if .WithInstructions}}Usa /reveal_{{.HiddenId}} en un grupo para compartirlo allí.{{end}}
    `,
	HIDDENSOURCEMSG:   "Mensaje oculto <code>{{.Id}}</code> revelado por {{.Revealers}}. Has recibido {{.Sats}} sat.",
	HIDDENREVEALMSG:   "{{.Sats}} sat  pagados para revelar el mensaje <code>{{.Id}}</code>.",
	HIDDENMSGNOTFOUND: "Mensaje oculto no encontrado.",
	HIDDENSHAREBTN:    "Compartir en otro chat",

	TOGGLEHELP: `Activa/desactiva las funciones de los bots en los grupos. En los supergrupos sólo puede ser ejecutado por los administradores.
      
/toggle_ticket_10 comienza a cobrar una cuota a todos los nuevos participantes. Útil como medida antispam. El dinero va al propietario del grupo.
/toggle_ticket deja de cobrar una tasa a los nuevos participantes. 
/toggle_language_ru cambia el idioma del chat al ruso, /toggle_language muestra el idioma del chat, estos también funcionan en los chats privados.
/toggle_spammy activa el modo 'spammy'. El modo 'spammy' está desactivado por defecto. Cuando está activado, las notificaciones de propinas se enviarán en el grupo en lugar de sólo en privado.
    `,

	SATS4ADSHELP: `
Sats4ads es un mercado de anuncios en Telegram. Paga dinero por mostrar anuncios a otros, recibe dinero por cada anuncio que veas.

Las tasas para cada usuario están en msatoshi-por-carácter. La tasa máxima es de 1000 msat.
Cada anuncio incluye también una tarifa fija de 1 sat.
Las imágenes y los vídeos se cotizan como si fueran 100 caracteres.
Los enlaces tienen un precio de 300 caracteres más, ya que tienen una molesta vista previa.

Para difundir un anuncio debes enviar un mensaje al bot que será el contenido de tu anuncio, y luego responderlo usando <code>/sats4ads broadcast ...</code> como se ha descrito. Puedes usar <code>--max-rate=500</code> y <code>--skip=0</code> para tener un mejor control sobre cómo se va a emitir su mensaje. Estos son los valores por defecto.

/sats4ads_on_15 pone tu cuenta en modo de lectura de anuncios. Cualquiera podrá publicarte mensajes por 15 msatoshi-por-carácter. Puedes ajustar ese precio.
/sats4ads_off desactiva tu cuenta para que no recibas más anuncios.
/sats4ads_rates muestra un desglose de cuántos nodos hay en cada nivel de precios. Útil para planificar su presupuesto publicitario con antelación.
/sats4ads_rate muestra tu tarifa.
/sats4ads_preview en respuesta a un mensaje, muestra una vista previa de cómo lo verán los demás usuarios. La cantidad de satoshis que se muestra en el mensaje de vista previa no es significativa.
/sats4ads_broadcast_1000 emite un anuncio. La última cifra es el número máximo de satoshis que se gastará. Los anuncios más baratos tendrán preferencia sobre los más caros. Debe emitirse en respuesta a otro mensaje, cuyo contenido se utilizará como texto del anuncio.
    `,
	SATS4ADSTOGGLE:    `#sats4ads {{if .On}}Ver anuncios y recibir {{printf "%.15g" .Sats}} sat por carácter.{{else}}No verás más anuncios.{{end}}`,
	SATS4ADSBROADCAST: `#sats4ads {{if .NSent}}Mensaje emitido {{.NSent}} tiempo{{s .NSent}} por un coste total de {{.Sats}} sat ({{dollar .Sats}}).{{else}}No se ha podido encontrar un homólogo al que notificar con los parámetros dados. /sats4ads_rates{{end}}`,
	SATS4ADSSTART:     `El mensaje está siendo emitiendo.`,
	SATS4ADSPRICETABLE: `#sats4ads Cantidad de usuarios <b>por</b> cada franja de precios.
{{range .Rates}}<code>{{.UpToRate}} msat</code>: <i>{{.NUsers}} usuario{{s .NUsers}}</i>
{{else}}
<i>Nadie está registrado para ver anuncios todavía.</i>
{{end}}
Cada anuncio cuesta los precios anteriores <i>per character</i> + <code>1 sat</code> por cada usuario.
    `,
	SATS4ADSADFOOTER: `[#sats4ads: {{printf "%.15g" .Sats}} sat]`,
	SATS4ADSVIEWED:   `Reclamar`,

	HELPHELP: "Muestra la ayuda completa o la ayuda sobre un comando específico.",

	STOPHELP: "El bot deja de mostrarte notificaciones.",

	PAYPROMPT: `
{{if .Sats}}<i>{{.Sats}} sat</i> ({{dollar .Sats}})
{{end}}{{if .Description}}<i>{{.Description}}</i>{{else}}<code>{{.DescriptionHash}}</code>{{end}}
{{if .ReceiverName}}
<b>Receptor</b>: {{.ReceiverName}}{{end}}
<b>Hash</b>: <code>{{.Hash}}</code>{{if ne .Currency "bc"}}
<b>Cadena</b>: {{.Currency}}{{end}}
<b>Creado</b>: {{.Created}}
<b>Expira</b>: {{.Expiry}}{{if .Expired}} <b>[EXPIRED]</b>{{end}}{{if .Hints}}
<b>Pistas</b>: {{range .Hints}}
- {{range .}}{{.ShortChannelId | channelLink}}: {{.PubKey | nodeAliasLink}}{{end}}{{end}}{{end}}
<b>Beneficiario</b>: {{.Payee | nodeLink}} (<u>{{.Payee | nodeAlias}}</u>)

{{if .Sats}}¿Pagar la factura descrita arriba?
{{else}}<b>Responda con la cantidad deseada para confirmar.</b>
{{end}}
    `,
	FAILEDDECODE: "Fallo en la decodificación de la factura: {{.Err}}",
	BALANCEMSG: `
<b>Saldo total</b>: {{printf "%.15g" .Sats}} sat ({{dollar .Sats}})
<b>Saldo disponible</b>: {{printf "%.15g" .Usable}} sat ({{dollar .Usable}})
<b>Total recibido</b>: {{printf "%.15g" .Received}} sat
<b>Total enviado</b>: {{printf "%.15g" .Sent}} sat
<b>Tarifas totales pagadas</b>: {{printf "%.15g" .Fees}} sat

#saldo
/transactions
    `,
	TAGGEDBALANCEMSG: `
<b>Total</b> <code>recibido - gastado</code> <b>en aplicaciones internas y de terceros -></b> /apps<b>:</b>

{{range .Balances}}<code>{{.Tag}}</code>: <i>{{printf "%.15g" .Balance}} sat</i>  ({{dollar .Balance}})
{{else}}
<i>Todavía no se ha realizado ninguna operación de etiquetado.</i>
{{end}}
#saldo
    `,
	FAILEDUSER: "No se pudo analizar el nombre del receptor.",
	LOTTERYMSG: `
¡Comienza una ronda de lotería!
Tarifa de entrada: {{.EntrySats}} sat
Total de participantes: {{.Participants}}
Premio: {{.Prize}}
Registrados: {{.Registered}}
    `,
	INVALIDPARTNUMBER: "Número inválido de participantes: {{.Number}}",
	USERSENTTOUSER:    "💛 {{menuItem .Sats .RawSats true }} ({{dollar .Sats}}) enviado(s) a {{.User}}{{if .ReceiverHasNoChat}} (no se ha podido notificar a{{.User}} ya que no ha iniciado una conversación con el bot){{end}}.",
	USERSENTYOUSATS:   "💛 {{.User}} te ha enviado {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}){{if .BotOp}} en un {{.BotOp}}{{end}}.",
	RECEIVEDSATSANON:  "💛 Alguien te ha enviado {{menuItem .Sats .RawSats false}} ({{dollar .Sats}}).",
	FAILEDSEND:        "Fallo de envío: ",
	QRCODEFAIL:        "Lectura de código QR fallida: {{.Err}}",
	SAVERECEIVERFAIL:  "No se ha podido guardar el receptor. Esto es probablemente un error.",
	MISSINGRECEIVER:   "¡Falta un receptor!",
	GIVERCANTJOIN:     "¡El donante no puede unirse!",
	CANTJOINTWICE:     "¡No puedes unirte dos veces.!",
	CANTREVEALOWN:     "¡No puedes revelar tu propio mensaje oculto!",
	CANTCANCEL:        "No tienes los poderes para cancelar esto.",
	FAILEDINVOICE:     "Fallo al generar la factura: {{.Err}}",
	STOPNOTIFY:        "Las notificaciones se detuvieron.",
	START: `
⚡️ @lntxbot, un monedero <b>Bitcoin</b> Lightning en tu Telegram.

🕹️  <b>Comandos básicos</b>
<b>&lt;invoice&gt;</b> -  Basta con pegar una factura (invoice) o una LNURL para descodificarla o pagarla.
<b>/balance</b> - Muestra tu saldo.
<b>/tip &lt;monto;&gt;</b> - Envía esto en respuesta a otro mensaje en un grupo para dar propina.
<b>/invoice &lt;monto&gt; &lt;descripción&gt;</b> - Genera una factura Lightning: <code>/invoice 400 'dividir cuenta del café'</code>.
<b>/send &lt;monto;&gt; &lt;usuario&gt;</b> - Envía algunos satoshis a otro usuario: <code>/send 100 @fiatjaf</code>

🍎 <b>Otras cosas que puedes hacer</b>
- Usa <b>/send</b> para enviar dinero a cualquier <a href="https://lightningaddress.com">Dirección Lightning</a>.
- Recibir dinero vía {{ .YourName }}@lntxbot.com o a través de https://lntxbot.com/@{{ .YourName }}.
- Hacer cálculos como <code>4*usd</code> o <code>eur*rand()</code> siempre que especifiques una cantidad en satoshis.
- Usa <b>/withdraw lnurl &lt;monto&gt;</b> para crear un vale LNURL-withdraw de retiro de fondos.

🎮 <b>Comandos divertidos o útiles</b>
<b>/sats4ads</b> Recibe dinero por ver mensajes de spam. Tú controlas la cantidad - o envía anuncios a todo el mundo. ¡Grandes tasas de conversión! 
<b>/giveaway</b> y <b>/giveflip</b> - ¡Regala dinero en grupo!
<b>/hide</b> - Oculta un mensaje; la gente tendrá que pagar para verlo. Múltiples formas de revelación: pública, privada, por campaña de recaudación. Soporta archivos multimedia.
<b>/coinflip &lt;monto&gt; &lt;número_de_participantes&gt;</b> - Crea una lotería en la que cualquiera puede participar <i>(cuesta 10 sat de comisión)</i>.

🐟 <b>Comandos Inline</b> - <i>Se puede utilizar en cualquier chat, incluso si el bot no está presente.</i>
<code>@lntxbot give &lt;monto&gt;</code> - Crea un botón en un chat privado para dar dinero a la otra parte.
<code>@lntxbot coinflip/giveflip/giveaway</code> - Igual que la versión del comando con barra, pero se puede utilizar en grupos sin @lntxbot.
<code>@lntxbot invoice &lt;monto&gt;</code> - Realiza una factura y la envía al chat.

🏖  <b>Comandos avanzados</b>
<b>/bluewallet</b> - Conecta BlueWallet o Zeus a tu cuenta @lntxbot.
<b>/transactions</b> - Enumera todas sus transacciones, en orden.
<b>/help &lt;comando;&gt;</b> - Muestra la ayuda detallada de un comando específico.
<b>/paynow &lt;factura&gt;</b> - Paga una factura sin pedirlo.
<b>/send --anonymous &lt;monto&gt; &lt;usuario&gt;</b> - El receptor no sabe quién le envía los sats.

🏛  <b>Administración de grupos</b>
<b>/toggle ticket &lt;monto&gt;</b> - Ponga un precio en satoshis para unirse a su grupo. ¡Gran antispam! El dinero va al dueño del grupo.
<b>/toggle renamable &lt;monto&gt;</b> - Permite que la gente use /rename para cambiar el nombre de tu grupo y te paguen.
<b>/toggle expensive &lt;monto&gt; &lt;palabras&gt;</b> - Cobra a la gente por decir las palabras incorrectas en tu grupo (o deja en blanco para cobrar por todos los mensajes).
<b>/fine &lt;amount&gt;</b> - Haz que la gente te pague o que te echen del grupo.

---

Hay otros comandos, pero su aprendizaje se deja como ejercicio al usuario.

¡Buena suerte! 🍽️
    `,
	WRONGCOMMAND:    "No se pudo entender el comando. /help",
	RETRACTQUESTION: "¿Retirar la propina no reclamada?",
	RECHECKPENDING:  "¿Revisar el pago pendiente?",

	TXNOTFOUND: "No se pudo encontrar la transacción {{.HashFirstChars}}.",
	TXINFO: `{{.Txn.Icon}} <code>{{.Txn.Status}}</code> {{.Txn.PeerActionDescription}} a {{.Txn.Time | time}} {{if .Txn.IsUnclaimed}}[💤 UNCLAIMED]{{end}}
<i>{{.Txn.Description}}</i>{{if .Txn.Tag.Valid}} #{{.Txn.Tag.String}}{{end}}{{if not .Txn.TelegramPeer.Valid}}
{{if .Txn.Payee.Valid}}<b>Beneficiario</b>: {{.Txn.Payee.String | nodeLink}} (<u>{{.Txn.Payee.String | nodeAlias}}</u>){{end}}
<b>Hash</b>: <code>{{.Txn.Hash}}</code>{{end}}{{if .Txn.Preimage.String}}
<b>Preimagen</b>: <code>{{.Txn.Preimage.String}}</code>{{end}}
<b>Monto</b>: <i>{{.Txn.Amount | printf "%.15g"}} sat</i> ({{dollar .Txn.Amount}})
{{if not (eq .Txn.Status "RECEIVED")}}<b>Tarifa pagada</b>: <i>{{printf "%.15g" .Txn.Fees}} sat</i>{{end}}
{{.LogInfo}}
    `,
	TXLIST: `<b>{{if .Offset}}Transacciones desde{{.From}} a {{.To}}{{else}}Últimas {{.Limit}} transacciones{{end}}</b>
{{range .Transactions}}<code>{{.StatusSmall}}</code> <code>{{.Amount | paddedSatoshis}}</code> {{.Icon}} {{.PeerActionDescription}}{{if not .TelegramPeer.Valid}}<i>{{.Description}}</i>{{end}} <i>{{.Time | timeSmall}}</i> /tx_{{.HashReduced}}
{{else}}
<i>Todavía no se ha realizado ninguna transacción.</i>
{{end}}
    `,
	TXLOG: `<b>Rutas probadas</b>{{if .PaymentHash}} para <code>{{.PaymentHash}}</code>{{end}}:
{{range $t, $try := .Tries}}{{if $try.Success}}✅{{else}}❌{{end}} {{range $h, $hop := $try.Route}}➠{{.Channel | channelLink}}{{end}}{{with $try.Error}}{{if $try.Route}}
{{else}} {{end}}<i>{{. | makeLinks}}</i>
{{end}}{{end}}
    `,
}
