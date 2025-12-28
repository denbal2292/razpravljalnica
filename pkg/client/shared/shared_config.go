package shared

import "time"

// Timeout duration for gRPC requests - common for all clients
// Different for subscriptions if needed
const Timeout = 5 * time.Second
