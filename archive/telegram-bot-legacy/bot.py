def main() -> None:
    """
    Entry point for Telegram bot.
    """
    import logging

    from dotenv import load_dotenv
    from telegram.ext import (
        Application,
        CallbackQueryHandler,
        CommandHandler,
        MessageHandler,
        PreCheckoutQueryHandler,
        ContextTypes,
        filters,
    )
    from telegram import Update

    from api_client import VPNProductClient
    from config import load_config
    from handlers import admin as admin_handlers
    from handlers import history as history_handlers
    from handlers import links as links_handlers
    from handlers import payment as payment_handlers
    from handlers import renew as renew_handlers
    from handlers import start as start_handlers
    from handlers import status as status_handlers
    from handlers import subscribe as subscribe_handlers
    from keyboards import main_menu
    from handlers.common import reply_text, is_admin, deny_if_not_allowed

    load_dotenv()
    logging.basicConfig(format="%(asctime)s - %(name)s - %(levelname)s - %(message)s", level=logging.INFO)
    logging.getLogger("httpx").setLevel(logging.WARNING)
    logging.getLogger("httpcore").setLevel(logging.WARNING)

    cfg = load_config()
    if not cfg.telegram_bot_token:
        raise SystemExit("TELEGRAM_BOT_TOKEN is required")
    if not cfg.dry_run:
        if not cfg.vpn_product_base_url:
            raise SystemExit("VPN_PRODUCT_BASE_URL is required")
        if not cfg.vpn_product_api_token:
            raise SystemExit("VPN_PRODUCT_API_TOKEN is required")

    app = Application.builder().token(cfg.telegram_bot_token).build()
    app.bot_data["cfg"] = cfg
    app.bot_data["allowed_ids"] = cfg.allowed_telegram_ids
    app.bot_data["admin_ids"] = cfg.admin_telegram_ids
    app.bot_data["profile_ids"] = cfg.vpn_profile_ids
    app.bot_data["payment_provider_token"] = cfg.payment_provider_token
    app.bot_data["payment_currency"] = cfg.payment_currency
    app.bot_data["payment_plans"] = cfg.payment_plans
    app.bot_data["payment_manual_details"] = cfg.payment_manual_details

    if cfg.dry_run:
        app.bot_data["vpn_client"] = None
    else:
        app.bot_data["vpn_client"] = VPNProductClient(cfg.vpn_product_base_url, cfg.vpn_product_api_token)

    async def on_callback(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        if await deny_if_not_allowed(update, context):
            return
        q = update.callback_query
        if not q:
            return
        await q.answer()
        data = (q.data or "").strip()
        uid = update.effective_user.id if update.effective_user else 0
        cfg = context.bot_data["cfg"]
        if data in ("menu", "cancel"):
            await reply_text(update, context, "Ок.", reply_markup=main_menu(is_admin(context, uid), cfg.miniapp_url))
            return
        if data == "help":
            await start_handlers.cmd_help(update, context)
            return
        if data == "subscribe":
            await subscribe_handlers.cmd_subscribe(update, context)
            return
        if data == "status":
            await status_handlers.cmd_status(update, context)
            return
        if data == "history":
            await history_handlers.cmd_history(update, context)
            return
        if data == "links":
            await links_handlers.cmd_links(update, context)
            return
        if data == "renew":
            await renew_handlers.cmd_renew(update, context)
            return
        if data == "pay":
            await payment_handlers.cmd_pay(update, context)
            return
        if data.startswith("pay_plan:"):
            try:
                months = int(data.split(":", 1)[1])
            except ValueError:
                months = 1
            await payment_handlers.on_pay_plan(update, context, months)
            return
        if data == "manual_paid":
            await payment_handlers.on_manual_paid(update, context)
            return
        if data.startswith("admin_confirm_payment:"):
            _, u, m = data.split(":")
            await payment_handlers.on_admin_confirm_payment(update, context, int(u), int(m))
            return
        if data == "admin":
            await admin_handlers.cmd_admin(update, context)
            return
        await reply_text(update, context, "Неизвестная команда.", reply_markup=main_menu(is_admin(context, uid), cfg.miniapp_url))

    async def on_precheckout(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
        if update.pre_checkout_query:
            await update.pre_checkout_query.answer(ok=True)

    app.add_handler(CommandHandler("start", start_handlers.cmd_start))
    app.add_handler(CommandHandler("help", start_handlers.cmd_help))
    app.add_handler(CommandHandler("subscribe", subscribe_handlers.cmd_subscribe))
    app.add_handler(CommandHandler("status", status_handlers.cmd_status))
    app.add_handler(CommandHandler("history", history_handlers.cmd_history))
    app.add_handler(CommandHandler("renew", renew_handlers.cmd_renew))
    app.add_handler(CommandHandler("links", links_handlers.cmd_links))
    app.add_handler(CommandHandler("pay", payment_handlers.cmd_pay))
    app.add_handler(CommandHandler("admin", admin_handlers.cmd_admin))
    app.add_handler(CommandHandler("admin_stats", admin_handlers.cmd_admin_stats))
    app.add_handler(CommandHandler("admin_health", admin_handlers.cmd_admin_health))
    app.add_handler(CommandHandler("admin_block", admin_handlers.cmd_admin_block))
    app.add_handler(CommandHandler("admin_unblock", admin_handlers.cmd_admin_unblock))

    app.add_handler(CallbackQueryHandler(on_callback))
    app.add_handler(PreCheckoutQueryHandler(on_precheckout))
    app.add_handler(MessageHandler(filters.SUCCESSFUL_PAYMENT, payment_handlers.on_successful_payment))
    app.add_handler(MessageHandler(filters.PHOTO, payment_handlers.on_payment_photo))

    app.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == "__main__":
    main()
