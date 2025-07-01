"""Unit tests for EventBus functionality."""

import asyncio
from unittest.mock import AsyncMock, MagicMock

import pytest

from beenet.core.events import Event, EventBus, EventType


class TestEvent:
    """Test Event class functionality."""

    def test_event_creation(self):
        """Test event creation."""
        event_type = EventType.PEER_CONNECTED
        data = {"peer_id": "test_peer"}
        source = "test_source"

        event = Event(event_type, data, source)

        assert event.event_type == event_type
        assert event.data == data
        assert event.source == source
        assert event.timestamp > 0

    def test_event_without_source(self):
        """Test event creation without source."""
        event_type = EventType.TRANSFER_STARTED
        data = {"transfer_id": "test_transfer"}

        event = Event(event_type, data)

        assert event.event_type == event_type
        assert event.data == data
        assert event.source is None


class TestEventBus:
    """Test EventBus functionality."""

    def test_eventbus_creation(self, event_bus):
        """Test EventBus creation."""
        assert event_bus._handlers == {}
        assert event_bus._global_handlers == []

    def test_subscribe_to_event_type(self, event_bus):
        """Test subscribing to specific event type."""
        handler = MagicMock()
        event_type = EventType.PEER_CONNECTED

        event_bus.subscribe(event_type, handler)

        assert event_type in event_bus._handlers
        assert handler in event_bus._handlers[event_type]

    def test_subscribe_to_all_events(self, event_bus):
        """Test subscribing to all events."""
        handler = MagicMock()

        event_bus.subscribe_all(handler)

        assert handler in event_bus._global_handlers

    def test_unsubscribe_from_event_type(self, event_bus):
        """Test unsubscribing from specific event type."""
        handler = MagicMock()
        event_type = EventType.PEER_DISCONNECTED

        event_bus.subscribe(event_type, handler)
        assert handler in event_bus._handlers[event_type]

        event_bus.unsubscribe(event_type, handler)
        assert handler not in event_bus._handlers[event_type]

    def test_unsubscribe_from_all_events(self, event_bus):
        """Test unsubscribing from all events."""
        handler = MagicMock()

        event_bus.subscribe_all(handler)
        assert handler in event_bus._global_handlers

        event_bus.unsubscribe_all(handler)
        assert handler not in event_bus._global_handlers

    @pytest.mark.asyncio
    async def test_publish_event_to_specific_handlers(self, event_bus):
        """Test publishing event to specific handlers."""
        handler = AsyncMock()
        event_type = EventType.TRANSFER_COMPLETED

        event_bus.subscribe(event_type, handler)

        event = Event(event_type, {"transfer_id": "test"})
        await event_bus.publish(event)

        handler.assert_called_once_with(event)

    @pytest.mark.asyncio
    async def test_publish_event_to_global_handlers(self, event_bus):
        """Test publishing event to global handlers."""
        handler = AsyncMock()

        event_bus.subscribe_all(handler)

        event = Event(EventType.KEY_ROTATED, {"peer_id": "test"})
        await event_bus.publish(event)

        handler.assert_called_once_with(event)

    @pytest.mark.asyncio
    async def test_emit_event(self, event_bus):
        """Test emitting event with data."""
        handler = AsyncMock()
        event_type = EventType.NETWORK_ERROR
        data = {"error": "connection failed"}
        source = "test_source"

        event_bus.subscribe(event_type, handler)

        await event_bus.emit(event_type, data, source)

        handler.assert_called_once()
        call_args = handler.call_args[0][0]
        assert call_args.event_type == event_type
        assert call_args.data == data
        assert call_args.source == source

    @pytest.mark.asyncio
    async def test_sync_handler(self, event_bus):
        """Test synchronous event handler."""
        handler = MagicMock()
        event_type = EventType.PEER_DISCOVERED

        event_bus.subscribe(event_type, handler)

        event = Event(event_type, {"peer_id": "test"})
        await event_bus.publish(event)

        handler.assert_called_once_with(event)

    @pytest.mark.asyncio
    async def test_handler_error_isolation(self, event_bus):
        """Test that handler errors don't affect other handlers."""
        good_handler = AsyncMock()
        bad_handler = AsyncMock(side_effect=Exception("Handler error"))
        event_type = EventType.TRANSFER_PROGRESS

        event_bus.subscribe(event_type, bad_handler)
        event_bus.subscribe(event_type, good_handler)

        event = Event(event_type, {"progress": 0.5})
        await event_bus.publish(event)

        bad_handler.assert_called_once_with(event)
        good_handler.assert_called_once_with(event)

    def test_clear_handlers(self, event_bus):
        """Test clearing all handlers."""
        handler1 = MagicMock()
        handler2 = MagicMock()

        event_bus.subscribe(EventType.PEER_CONNECTED, handler1)
        event_bus.subscribe_all(handler2)

        event_bus.clear_handlers()

        assert event_bus._handlers == {}
        assert event_bus._global_handlers == []

    def test_get_handler_count(self, event_bus):
        """Test getting handler count."""
        handler1 = MagicMock()
        handler2 = MagicMock()
        handler3 = MagicMock()

        event_bus.subscribe(EventType.PEER_CONNECTED, handler1)
        event_bus.subscribe(EventType.PEER_CONNECTED, handler2)
        event_bus.subscribe_all(handler3)

        assert event_bus.get_handler_count(EventType.PEER_CONNECTED) == 2
        assert event_bus.get_handler_count(EventType.PEER_DISCONNECTED) == 0
        assert event_bus.get_handler_count(None) == 1

    @pytest.mark.asyncio
    async def test_multiple_event_types(self, event_bus):
        """Test handling multiple event types."""
        handler1 = AsyncMock()
        handler2 = AsyncMock()

        event_bus.subscribe(EventType.PEER_CONNECTED, handler1)
        event_bus.subscribe(EventType.TRANSFER_STARTED, handler2)

        await event_bus.emit(EventType.PEER_CONNECTED, {"peer_id": "test1"})
        await event_bus.emit(EventType.TRANSFER_STARTED, {"transfer_id": "test2"})

        handler1.assert_called_once()
        handler2.assert_called_once()

    @pytest.mark.asyncio
    async def test_no_handlers_for_event(self, event_bus):
        """Test publishing event with no handlers."""
        event = Event(EventType.TRANSFER_FAILED, {"error": "timeout"})

        await event_bus.publish(event)
