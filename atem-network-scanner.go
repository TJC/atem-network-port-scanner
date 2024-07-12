package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type response struct {
	Success   bool     `json:"success"`
	Addresses []string `json:"addresses"`
}

func usage() {
	fmt.Println("Usage: atem-network-scanner --interface eth0")
	fmt.Println("That will take the IP address from eth0 and scan the adjacent 254 IPs to see if any are an ATEM.")
	os.Exit(1)
}

func main() {
	interfaceName := flag.String("interface", "", "Interface to query for our own IP address")
	flag.Parse()

	if *interfaceName == "" {
		usage()
	}

	addressQueue := make(chan string, 255)
	results := make(chan string, 255)
	var wg sync.WaitGroup

	// Get the list of IP addresses in the local network range
	netInterface, err := net.InterfaceByName(*interfaceName)
	if err != nil {
		fmt.Println("Couldn't find that interface: ", err)
		os.Exit(1)
	}
	addrs, _ := netInterface.Addrs()
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && ipNet.IP.To4() != nil {
			myIp := ipNet.IP.To4()
			// fmt.Fprintf(os.Stderr, "This device IP: %s", myIp)
			genIpRange(myIp, addressQueue)
		}
	}
	close(addressQueue)

	for i := 1; i <= 8; i++ {
		wg.Add(1)
		go pingWorker(addressQueue, results, &wg)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	foundAddrs := []string{}

	for foundAddr := range results {
		foundAddrs = append(foundAddrs, foundAddr)
	}

	r, _ := json.Marshal(response{
		Success:   true,
		Addresses: foundAddrs,
	})
	fmt.Println(string(r))

	os.Exit(0)
}

// Worker that attempts to connect to any host from the queue.
// Upon finding a host, it pops it back in the results queue
func pingWorker(addressQueue <-chan string, results chan<- string, wg *sync.WaitGroup) {
	for addr := range addressQueue {
		if pingATEM(addr) {
			results <- addr
		}
	}
	wg.Done()
}

// Generates an IP range based on the given address, and just pops them all into a channel
func genIpRange(myIp net.IP, addressQueue chan<- string) {
	thisDevice := make(net.IP, len(myIp))
	copy(thisDevice, myIp)

	targetAddr := myIp
	for i := 1; i <= 255; i++ {
		targetAddr[3] = byte(i)
		if !targetAddr.Equal(thisDevice) {
			addressQueue <- targetAddr.String()
		}
	}
}

func pingATEM(addr string) bool {
	udpAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:9910", addr))
	conn, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Duration(100 * time.Millisecond)))

	header := []byte{0x10, 0x14, 0x13, 0x37, 0, 0, 0, 0, 0, 0, 0, 0}
	payload := []byte{1, 0, 0, 0, 0, 0, 0, 0}
	message := append(header, payload...)

	_, err = conn.Write(message)
	if err != nil {
		fmt.Print(err)
		return false
	}

	p := make([]byte, 64)
	_, err = conn.Read(p)
	return err == nil
}
