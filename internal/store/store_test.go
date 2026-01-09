package store // Declares this file as part of the 'store' package, allowing it to test the store package's functionality.

import ( // Import block starts here, bringing in external packages needed for testing.
	"os"      // Package for operating system interface functions, used here to remove test files.
	"testing" // Package providing testing support and the testing.T type for writing test functions.

	"github.com/mathdee/KV-Store/internal/wal" // Imports the WAL package to test integration between Store and WAL functionality.
) // Import block ends here.

func TestStore(t *testing.T) { // Test function: 't *testing.T' is a pointer to a testing.T struct - the * means we receive a pointer, allowing the test framework to track test state and report failures.
	// Setup a temp WAL file

	filename := "test_wal.log" // Declares a string variable 'filename' and assigns it the name of the temporary WAL log file used for testing.
	os.Remove(filename)        //clean up previous runs
	defer os.Remove(filename)  //always clean up after test is run.
	// 'defer' schedules this function call to execute when TestStore returns, ensuring cleanup happens even if the test fails.

	// Initialize WAL

	w, err := wal.NewWAL(filename) // Calls NewWAL with the filename: 'w' receives a pointer to a WAL instance (*wal.WAL), and 'err' receives any error that occurred. The := operator declares and assigns both variables.
	if err != nil {                // Checks if the error value is not nil, meaning an error occurred during WAL creation.
		t.Fatalf("Failed to create WAL: %v", err) // Calls Fatalf on the test pointer 't' - the * in the receiver allows this method to modify test state. Fatalf logs the error and immediately stops test execution.
	} // End of error check block.

	// Create Store and write data
	s := NewStore(w)         // Creates a new Store instance: 's' receives a pointer to Store (*Store) returned by NewStore. The 'w' parameter (a pointer to WAL) is passed to initialize the Store with WAL functionality.
	s.Set("user", "Mathijs") // Calls the Set method on the Store pointer 's' to store a key-value pair. Since 's' is a pointer, the method can modify the Store's internal data.
	w.Close()                // simulates server shutdown
	// Calls Close on the WAL pointer 'w' to close the file, simulating what happens when the server shuts down.

	// Simulate restart (reads file from disk)
	recoveredData, err := wal.Recover(filename) // Calls the Recover function (not a method, so no pointer receiver) to read the WAL file and reconstruct the data map from disk.
	if err != nil {                             // Checks if an error occurred during recovery.
		t.Fatalf("Failed to recover: %v", err) // Logs the error and stops test execution if recovery failed.
		// The %v verb formats the error value for display in the test output.

	} // End of error check block.

	// create a fresh store with recovered data
	w2, _ := wal.NewWAL(filename) // Creates a new WAL instance: 'w2' receives the pointer, and '_' (blank identifier) discards the error return value, ignoring potential errors for this test scenario.
	s2 := NewStore(w2)            // Creates a new Store instance 's2' with the new WAL pointer 'w2', simulating a fresh server instance after restart.
	s2.Restore(recoveredData)     // Calls Restore on Store pointer 's2' to populate its data map with the recovered data from the WAL file.

	// Verify if data is back
	val, err := s2.Get("user") // Calls Get method on Store pointer 's2' to retrieve the value for key "user": 'val' receives the string value, 'err' receives any error.
	if err != nil {            // Checks if an error occurred while retrieving the value, which would indicate the key was not found.
		t.Fatalf("Failed to get value: %v", err) // Logs the error and stops test execution if Get returned an error, since we expect the key to exist.
	} // End of error check block.
	if val != "Mathijs" { // Compares the retrieved value with the expected value "Mathijs" (note: case-sensitive comparison).
		t.Errorf("Expected Mathijs, got %s", val) // Calls Errorf on test pointer 't' to log a test failure if the value doesn't match, but continues test execution (unlike Fatalf).
	} // End of value comparison block.

} // End of TestStore function.

// func TestStore(t *testing.T) {
//
// 	s := NewStore()
//
// 	s.Set("name", "Mathijs")
//
// 	val, err := s.Get("name")
// 	if err != nil {
// 		t.Fatalf("Expected no error, got %v", err)
// 	}
// 	if val != "Mathijs" {
// 		t.Errorf("Expected Mathijs, got %s", val)
// 	}
//
// 	_, err = s.Get("missing key")
// 	if err != ErrorNotFound {
// 		t.Errorf("Expected ErrorNotFound, got %v", err)
// 	}
// }
