package store // Declares this file as part of the 'store' package, making it accessible to other packages that import it.

import ( // Import block starts here, bringing in external packages needed by this file.
	"errors" // Package for creating and handling error values in Go.
	"sync"   // Package providing synchronization primitives like mutexes for concurrent programming.

	"github.com/mathdee/KV-Store/internal/wal" // Imports the WAL (Write-Ahead Log) package from the internal directory to use WAL functionality.
) // Import block ends here.

var ErrorNotFound = errors.New("key not found") // custom error variable to return when key is not found.

type Store struct { //Store struct to store data.
	mu   sync.RWMutex      // a read-write mutex that allows multiple readers OR a single writer.
	wal  *wal.WAL          // Pointer (*) to a WAL struct - the * means this field stores the memory address of a WAL instance, not the WAL itself. This allows sharing the same WAL instance across multiple Store instances if needed.
	data map[string]string // a map of String keys to String values.

} // End of Store struct definition.

func NewStore(w *wal.WAL) *Store { // Constructor function: 'w *wal.WAL' means it takes a pointer to a WAL as a parameter (the * indicates a pointer type). The return type '*Store' means it returns a pointer to a Store instance (not the Store value itself).
	return &Store{ // The & operator gets the memory address of the newly created Store struct literal, returning a pointer to it. This allows the caller to work with the same Store instance in memory.
		data: make(map[string]string), //initialize the map with a size of 0 and capacity of 100.
		wal:  w,                       // Assigns the WAL pointer parameter 'w' to the Store's wal field, storing the memory address of the WAL instance.
	} // End of struct literal initialization.
} // End of NewStore function.

func (s *Store) Set(key string, value string) error { // Method on Store: '(s *Store)' is a pointer receiver - the * means this method receives a pointer to a Store instance, allowing it to modify the Store's fields directly. Returns an error type to indicate success or failure.
	if err := s.wal.WriteEntry(key, value); err != nil { // Calls WriteEntry on the WAL instance (accessed through the pointer s.wal) and checks if it returned an error.
		return err // Returns the error immediately if WAL write failed, stopping further execution.
	} // End of error check block.

	s.mu.Lock()         // Locks out all readers and writers until finished.
	s.data[key] = value // Stores the key-value pair in the in-memory map, using the key as the index and value as the stored data.
	defer s.mu.Unlock() // Defers the unlock operation to execute when the function returns, ensuring the mutex is always released even if an error occurs.
	return nil          // Returns nil to indicate the operation completed successfully without errors.
} // End of Set method.

func (s *Store) Get(key string) (string, error) { //Get method to find a value by its key.

	s.mu.RLock()         //lock mutex when reading the data.
	defer s.mu.RUnlock() // unlock mutex when the function returns.

	val, ok := s.data[key] //this check if the key exists in the map.
	if !ok {               // and if the key does not exist it return ErrorNotFound.
		return "", ErrorNotFound // if not exist, return empty string and ErrorNotFound.
	} // End of error check block.
	return val, nil // if key exists, returns value and nil error.
} // End of Get method.

func (s *Store) Restore(data map[string]string) { // Method with pointer receiver '(s *Store)' - allows modifying the Store's data field directly through the pointer.
	s.mu.Lock()         // Acquires an exclusive write lock on the mutex to prevent other goroutines from reading or writing while we modify the data.
	defer s.mu.Unlock() // Ensures the mutex is unlocked when the function exits, even if an error occurs.
	s.data = data       // Replaces the entire data map with the provided map, restoring the Store's state from the WAL recovery process.
} // End of Restore method.
