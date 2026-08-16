"""Vendor-neutral All-Chat client used by the StreamController actions.

Nothing in this package imports StreamController, GTK or any plugin host API, so
it can be exercised by ``python3 -m unittest`` on a machine with no Stream Deck
attached -- which is the only way most of this code gets tested, since the real
host only exists on a streamer's Linux desktop.

The split is deliberate:

* :mod:`allchat.settings` -- defaults, validation, no I/O.
* :mod:`allchat.errors`   -- the error taxonomy, including the three-way split of
  HTTP 403 into premium gate / missing scope / other.
* :mod:`allchat.client`   -- one ``urllib`` request function; the only place the
  token is ever used.
* :mod:`allchat.api`      -- the endpoint list, with the premium flag beside the
  path it belongs to.
"""

from __future__ import annotations

__all__ = ["api", "client", "errors", "settings"]
