// Package rpc provides WebSocket-based RPC functionality.
package rpc

// RPC method constants
const (
	// Room methods
	MethodRoomCreate       = "room.create"
	MethodRoomGet          = "room.get"
	MethodRoomGetBySlug    = "room.getBySlug"
	MethodRoomUpdate       = "room.update"
	MethodRoomDelete       = "room.delete"
	MethodRoomJoin         = "room.join"
	MethodRoomLeave        = "room.leave"
	MethodRoomGetUsers     = "room.getUsers"
	MethodRoomIsUserInRoom = "room.isUserInRoom"
	MethodRoomGetState     = "room.getState"
	MethodRoomSearch       = "room.search"
	MethodRoomGetActive    = "room.getActive"
	MethodRoomGetPopular   = "room.getPopular"

	// Queue methods
	MethodQueueJoin            = "queue.join"
	MethodQueueLeave           = "queue.leave"
	MethodQueueMove            = "queue.move"
	MethodQueueGet             = "queue.get"
	MethodQueueGetCurrentDJ    = "queue.getCurrentDJ"
	MethodQueueGetCurrentMedia = "queue.getCurrentMedia"
	MethodQueueAdvance         = "queue.advance"
	MethodQueuePlayMedia       = "queue.playMedia"
	MethodQueueSkip            = "queue.skip"
	MethodQueueClear           = "queue.clear"
	MethodQueueShuffle         = "queue.shuffle"
	MethodQueueGetPosition     = "queue.getPosition"
	MethodQueueIsInQueue       = "queue.isInQueue"
	MethodQueueIsCurrentDJ     = "queue.isCurrentDJ"
	MethodQueueGetHistory      = "queue.getHistory"

	// Media methods
	MethodMediaSearch       = "media.search"
	MethodMediaGetInfo      = "media.getInfo"
	MethodMediaGetStreamURL = "media.getStreamURL"

	// Chat methods
	MethodChatSendMessage   = "chat.sendMessage"
	MethodChatGetMessages   = "chat.getMessages"
	MethodChatDeleteMessage = "chat.deleteMessage"
)

// RPC event constants
const (
	// Room events
	EventUserJoinedRoom   = "user:room_join"
	EventUserLeftRoom     = "user:room_leave"
	EventRoomStateChanged = "room:state_changed"
	EventRoomUpdated      = "room:updated"

	// Queue events
	EventQueueUpdated = "queue:updated"
	EventTrackStart   = "track:start"
	EventTrackEnd     = "track:end"
	EventTrackSkip    = "track:skip"
	EventTrackVote    = "track:vote"

	// Chat events
	EventChatMessage       = "chat:message"
	EventChatMessageDelete = "chat:message_delete"
)

// RPC error codes
const (
	ErrorCodeRoomNotFound      = 4001
	ErrorCodeRoomFull          = 4002
	ErrorCodeRoomClosed        = 4003
	ErrorCodeUserNotInRoom     = 4004
	ErrorCodeUserAlreadyInRoom = 4005
	ErrorCodeQueueFull         = 4006
	ErrorCodeNotCurrentDJ      = 4007
	ErrorCodeInvalidVote       = 4008
)
