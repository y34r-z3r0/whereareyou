package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

type NetworkInfo struct {
	ESSID     string
	MAC       string
	Channel   int
	Signal    int
	Decibel   int
	Frequency int
	Distance  float64
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter Wi-Fi interface name (e.g., wlan0, wlp2s0): ")
	interfaceName, _ := reader.ReadString('\n')
	interfaceName = strings.TrimSpace(interfaceName)

	if !checkSudo() {
		fmt.Println("This script requires sudo privileges for iwlist")
		fmt.Println("Please run with: sudo go run main.go")
		os.Exit(1)
	}

	if !checkInterface(interfaceName) {
		fmt.Printf("Error: interface %s not found or unavailable\n", interfaceName)
		fmt.Println("Available interfaces:")
		listInterfaces()
		os.Exit(1)
	}

	fmt.Printf("Starting scanning on interface %s every 5 seconds...\n\n", interfaceName)
	fmt.Println("Press Ctrl+C to exit")

	sniffer(interfaceName)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sniffer(interfaceName)
	}
}

func checkSudo() bool {
	cmd := exec.Command("sudo", "-n", "true")
	err := cmd.Run()
	return err == nil
}

func sniffer(interfaceName string) {
	cmd := exec.Command("sudo", "iwlist", interfaceName, "scan")
	output, err := cmd.Output()

	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")

	var networks []NetworkInfo
	var currentNet NetworkInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "Address:") {
			if currentNet.MAC != "" {
				networks = append(networks, currentNet)
			}
			currentNet = NetworkInfo{}

			re := regexp.MustCompile(`Address:\s+(\S+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				currentNet.MAC = matches[1]
			}
		}

		if strings.Contains(line, "ESSID:") {
			re := regexp.MustCompile(`ESSID:"(.*)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				currentNet.ESSID = matches[1]
			}
		}

		if strings.Contains(line, "Channel:") {
			re := regexp.MustCompile(`Channel:(\d+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				channel, _ := strconv.Atoi(matches[1])
				currentNet.Channel = channel
				currentNet.Frequency = channel*5 + 2407
			}
		}

		if strings.Contains(line, "Signal level=") {
			re := regexp.MustCompile(`Signal level=(-?\d+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				signal, _ := strconv.Atoi(matches[1])
				currentNet.Signal = signal
				currentNet.Decibel = int(math.Abs(float64(signal)))
			}
		}
	}

	if currentNet.MAC != "" {
		networks = append(networks, currentNet)
	}

	for i := range networks {
		if networks[i].Frequency > 0 && networks[i].Decibel > 0 {
			freqFloat := float64(networks[i].Frequency)
			dbFloat := float64(networks[i].Decibel)

			exponent := (27.55 - (20 * math.Log10(freqFloat)) + dbFloat) / 20
			networks[i].Distance = math.Pow(10, exponent)
		}
	}

	sort.Slice(networks, func(i, j int) bool {
		return networks[i].Signal > networks[j].Signal
	})

	clearScreen()
	printTable(networks)
}

func printTable(networks []NetworkInfo) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ESSID", "MAC Address", "Channel", "Signal", "Distance", "Frequency", "Decibel"})

	for _, net := range networks {
		if net.ESSID == "" {
			net.ESSID = "<hidden network>"
		}

		distance := fmt.Sprintf("%.2f m", net.Distance)
		if net.Distance == 0 {
			distance = "N/A"
		}

		t.AppendRow(table.Row{
			net.ESSID,
			net.MAC,
			net.Channel,
			net.Signal,
			distance,
			net.Frequency,
			net.Decibel,
		})
	}

	t.SetStyle(table.StyleRounded)
	fmt.Println(t.Render())
	fmt.Printf("\nScan time: %s\n", time.Now().Format("15:04:05"))
}

func checkInterface(interfaceName string) bool {
	cmd := exec.Command("ip", "link", "show", interfaceName)
	err := cmd.Run()
	return err == nil
}

func listInterfaces() {
	cmd := exec.Command("ip", "-br", "link", "show")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Failed to get interface list")
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "wl") || strings.Contains(line, "wlan") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				fmt.Printf("  - %s\n", fields[0])
			}
		}
	}
}

func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}
