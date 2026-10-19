package protomesh

// ResetDeprecationWarnings forgets which deprecated keys have been logged.
func ResetDeprecationWarnings() { warned.Clear() }
