package main

import ("fmt"; "sync"; _"time" ) // " _" is used to import a package but not use it.

func main(){

	var wg sync.WaitGroup // used to wait for a collection of goroutines to finish executing.
	counter := 0
	var mu sync.Mutex  
     // is a mutual exclusion lock used to protect shared data from being accessed by multiple 
	 // goroutines simultaneously, which prevents race conditions. 

	 for i:= 0; i < 1000; i++{
		wg.Add(1) // increment the wait group counter by 1.
		go func(){
			defer wg.Done() //decrement the wait group counter by 1
			mu.Lock() //lock the mutex to prevent race conditions
			counter++ // increment the counter
			mu.Unlock()// unlock the mutex to allow other goroutines to access the shared data
		}()
	 }

	 wg.Wait() // wait for all goroutines to finish executing
	 fmt.Println("Final Counter: ", counter) //print the final value of the counter

}