package client

import "time"

// Timeout duration for gRPC requests - common for all clients
// Different for subscriptions if needed
const timeout = 5 * time.Second
