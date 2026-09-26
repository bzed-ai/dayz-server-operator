# Sphinx configuration for the dzo documentation.
# Only Sphinx itself (python3-sphinx) is needed; no extensions, built-in theme.

project = "dzo"
copyright = "dayz-server-operator contributors"
author = "dayz-server-operator contributors"
release = "0.0"

extensions = []
exclude_patterns = ["_build"]
language = "en"

html_theme = "alabaster"
html_title = "dzo documentation"
html_theme_options = {
    "description": "DayZ server operator",
    "fixed_sidebar": True,
    "show_relbars": True,
}
html_show_sourcelink = False
