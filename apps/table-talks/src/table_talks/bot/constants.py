"""Constants used across the bot."""

# Callback data prefixes/patterns
CALLBACK_THEME_PREFIX = "theme:"
CALLBACK_NEXT = "next"
CALLBACK_PREVIOUS = "previous"
CALLBACK_NEW_TOPIC = "new_topic"
CALLBACK_END_SESSION = "end_session"

# Home page actions
CALLBACK_HOME = "home"
CALLBACK_START_SESSION = "start_session"
CALLBACK_BOT_INFO = "bot_info"
CALLBACK_SUPPORT = "support"
CALLBACK_EXIT = "exit"
CALLBACK_BACK_TO_HOME = "back_to_home"
CALLBACK_RANDOM_MIX = "random_mix"

# Bot data keys
CHAT_IDS_KEY = "chat_ids"

# Messages
OFFLINE_MESSAGE = "The bot is going offline. Try again later."

HOME_WELCOME_MESSAGE = """Welcome to Table Talks! 🎲

This bot helps you spark meaningful conversations with curated question cards organized by themes.

How it works:
1. Choose a theme
2. Get random questions one at a time
3. Discuss and enjoy quality time together

Ready to start?"""

BOT_INFO_MESSAGE = """ℹ️ Bot Information

Table Talks helps create engaging conversations through curated question cards.

📊 Version: {version}
🕐 Last Updated: {last_updated}

📝 Recent Changes:
{changelog}"""

SUPPORT_CREATOR_MESSAGE = """☕ Support the Creator

Thank you for considering supporting Table Talks!

Your contribution helps maintain and improve the bot.

Buy me a coffee: {coffee_link}"""

SUPPORT_UNAVAILABLE_MESSAGE = """☕ Support Feature

This feature is only available to the bot creator for testing.

Stay tuned for public availability!"""

EXIT_MESSAGE = "Thanks for using Table Talks! Send /start anytime to return."

# Configuration defaults (can be overridden by env vars)
DEFAULT_BOT_VERSION = "v0.1.0"
