"""Event loop integration and message routing for beenet."""

import asyncio
from enum import Enum
from typing import Any, Callable, Dict, List, Optional


class EventType(Enum):
    """Types of events in the beenet system."""

    PEER_DISCOVERED = "peer_discovered"
    PEER_CONNECTED = "peer_connected"
    PEER_DISCONNECTED = "peer_disconnected"
    TRANSFER_STARTED = "transfer_started"
    TRANSFER_PROGRESS = "transfer_progress"
    TRANSFER_COMPLETED = "transfer_completed"
    TRANSFER_FAILED = "transfer_failed"
    KEY_ROTATED = "key_rotated"
    NETWORK_ERROR = "network_error"


class Event:
    """Represents an event in the beenet system."""

    def __init__(self, event_type: EventType, data: Dict[str, Any], source: Optional[str] = None):
        self.event_type = event_type
        self.data = data
        self.source = source
        self.timestamp = asyncio.get_event_loop().time()


class EventBus:
    """Asyncio-based event handling and message routing.

    Provides:
    - Event subscription and publishing
    - Async event handlers
    - Event filtering and routing
    - Error isolation between handlers
    """

    def __init__(self):
        self._handlers: Dict[EventType, List[Callable[[Event], Any]]] = {}
        self._global_handlers: List[Callable[[Event], Any]] = []
        self._lock = asyncio.Lock()

    def subscribe(self, event_type: EventType, handler: Callable[[Event], Any]) -> None:
        """Subscribe to events of a specific type.

        Args:
            event_type: Type of events to subscribe to
            handler: Function to call when event occurs
        """
        if event_type not in self._handlers:
            self._handlers[event_type] = []
        self._handlers[event_type].append(handler)

    def subscribe_all(self, handler: Callable[[Event], Any]) -> None:
        """Subscribe to all events.

        Args:
            handler: Function to call for any event
        """
        self._global_handlers.append(handler)

    def unsubscribe(self, event_type: EventType, handler: Callable[[Event], Any]) -> None:
        """Unsubscribe from events.

        Args:
            event_type: Type of events to unsubscribe from
            handler: Handler function to remove
        """
        if event_type in self._handlers and handler in self._handlers[event_type]:
            self._handlers[event_type].remove(handler)

    def unsubscribe_all(self, handler: Callable[[Event], Any]) -> None:
        """Unsubscribe from all events.

        Args:
            handler: Handler function to remove
        """
        if handler in self._global_handlers:
            self._global_handlers.remove(handler)

    async def publish(self, event: Event) -> None:
        """Publish an event to all subscribers.

        Args:
            event: Event to publish
        """
        async with self._lock:
            if event.event_type in self._handlers:
                for handler in self._handlers[event.event_type]:
                    try:
                        if asyncio.iscoroutinefunction(handler):
                            await handler(event)
                        else:
                            handler(event)
                    except Exception as e:
                        pass

            for handler in self._global_handlers:
                try:
                    if asyncio.iscoroutinefunction(handler):
                        await handler(event)
                    else:
                        handler(event)
                except Exception as e:
                    pass

    async def emit(
        self, event_type: EventType, data: Dict[str, Any], source: Optional[str] = None
    ) -> None:
        """Emit an event with the given data.

        Args:
            event_type: Type of event to emit
            data: Event data
            source: Optional source identifier
        """
        event = Event(event_type, data, source)
        await self.publish(event)

    def clear_handlers(self) -> None:
        """Clear all event handlers."""
        self._handlers.clear()
        self._global_handlers.clear()

    def get_handler_count(self, event_type: Optional[EventType] = None) -> int:
        """Get number of handlers for an event type.

        Args:
            event_type: Event type to count handlers for (None for global)

        Returns:
            Number of handlers
        """
        if event_type is None:
            return len(self._global_handlers)
        return len(self._handlers.get(event_type, []))
