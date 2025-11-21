package queue

// This file is intentionally small: the round-robin algorithm is implemented
// inside queue_manager.go. If you'd like to experiment with alternate
// strategies (weighted RR, fair queuing), implement them here and expose
// a strategy interface. Kept separate to make future extension clear.
