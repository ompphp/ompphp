# Network API example

This example observes incoming RPC 24 through the managed standalone-CAPI bridge. Network handlers run synchronously on the main gamemode runtime. Return `NetworkResult::DROP` to stop the message or call `$message->buffer->replace()` to replace its payload before processing continues.

Use protocol IDs and payload formats appropriate for the active open.mp network implementation. The SDK intentionally does not guess packet layouts.
