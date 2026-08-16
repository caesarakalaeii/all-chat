"""Adapter over the StreamController host API.

StreamController plugins import their base classes from the running application::

    from src.backend.PluginManager.ActionBase import ActionBase
    from src.backend.PluginManager.ActionHolder import ActionHolder
    from src.backend.PluginManager.PluginBase import PluginBase

Those modules exist only *inside* a StreamController process -- ``src`` is the
application's own package, injected on ``sys.path`` when it loads a plugin. There
is no PyPI distribution to depend on, which is why ``requirements.txt`` is empty
of runtime packages and why importing this plugin's modules would otherwise fail
on any machine that is not running StreamController.

So: import the real classes when they are there, and fall back to minimal
stand-ins when they are not. The fallbacks exist for two concrete reasons:

1. ``python3 -m compileall`` and the unit tests must pass in CI, which has no
   Stream Deck and no StreamController.
2. A contributor can exercise the request and error-mapping logic on a laptop.

The stand-ins record what was asked of them instead of drawing anything, and are
never used at runtime on a real install -- :data:`HOST_AVAILABLE` says which case
you are in.
"""

from __future__ import annotations

from typing import Any

try:  # pragma: no cover - the real host is absent in CI
    from src.backend.PluginManager.ActionBase import ActionBase  # type: ignore
    from src.backend.PluginManager.ActionHolder import ActionHolder  # type: ignore
    from src.backend.PluginManager.PluginBase import PluginBase  # type: ignore

    HOST_AVAILABLE = True
except Exception:  # noqa: BLE001 - any import failure means "not in the host"
    HOST_AVAILABLE = False

    class ActionBase:  # type: ignore[no-redef]
        """Stand-in for StreamController's ``ActionBase``.

        Mirrors only the surface :mod:`actions.base` touches. Every draw call is
        recorded on the instance so a test can assert what the key would show.
        """

        def __init__(self, *args: Any, **kwargs: Any) -> None:
            self.plugin_base = kwargs.get("plugin_base")
            self._settings: dict[str, Any] = dict(kwargs.get("settings") or {})
            # Recorded calls, for tests.
            self.center_label: str = ""
            self.bottom_label: str = ""
            self.errors_shown: int = 0
            self.infos_shown: list[str] = []

        # -- settings store --
        def get_settings(self) -> dict[str, Any]:
            return self._settings

        def set_settings(self, settings: dict[str, Any]) -> None:
            self._settings = dict(settings)

        # -- rendering --
        def set_center_label(self, text: str, **_: Any) -> None:
            self.center_label = text

        def set_bottom_label(self, text: str, **_: Any) -> None:
            self.bottom_label = text

        def set_media(self, **_: Any) -> None:
            pass

        def show_error(self, *_: Any, **__: Any) -> None:
            self.errors_shown += 1

        def show_info(self, text: str = "", *_: Any, **__: Any) -> None:
            self.infos_shown.append(text)

    class ActionHolder:  # type: ignore[no-redef]
        """Stand-in for StreamController's ``ActionHolder``.

        The real one is what registers an action with the host: it carries the
        action's ID, its class, and the name shown in the action picker.
        """

        def __init__(self, **kwargs: Any) -> None:
            self.plugin_base = kwargs.get("plugin_base")
            self.action_base = kwargs.get("action_base")
            self.action_id = kwargs.get("action_id", "")
            self.action_name = kwargs.get("action_name", "")
            self.kwargs = kwargs

    class PluginBase:  # type: ignore[no-redef]
        """Stand-in for StreamController's ``PluginBase``."""

        def __init__(self, *args: Any, **kwargs: Any) -> None:
            self.action_holders: dict[str, Any] = {}
            self.logger = _FallbackLogger()

        def add_action_holder(self, holder: Any) -> None:
            self.action_holders[getattr(holder, "action_id", "")] = holder

        def register(self, *args: Any, **kwargs: Any) -> None:
            pass

    class _FallbackLogger:
        """Logger used only outside the host. Prefixed so it is obvious.

        Never receives a token: callers pass messages built by
        :mod:`allchat.errors`, which never interpolate the secret.
        """

        def info(self, message: str) -> None:
            print(f"[all-chat] INFO  {message}")

        def warning(self, message: str) -> None:
            print(f"[all-chat] WARN  {message}")

        def error(self, message: str) -> None:
            print(f"[all-chat] ERROR {message}")


__all__ = ["ActionBase", "ActionHolder", "PluginBase", "HOST_AVAILABLE"]
