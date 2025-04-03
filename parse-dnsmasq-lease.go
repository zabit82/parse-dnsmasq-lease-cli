package main

import (
	"bufio"          // For reading the file line by line
	"fmt"            // For formatted output
	"log"            // For logging errors
	"os"             // For file operations, environment variables, and standard output
	"strconv"        // For converting string to number (timestamp)
	"strings"        // For splitting strings
	"text/tabwriter" // For formatting output as a table
	"time"           // For time operations
)

// LeaseEntry represents a single DHCP lease record
type LeaseEntry struct {
	ExpiryTime time.Time // Lease expiration time
	MACAddress string    // Client MAC address
	IPAddress  string    // Assigned IP address
	Hostname   string    // Client hostname (can be '*')
	ClientID   string    // Client identifier (can be '*')
}

const defaultLeaseFilePath = "/var/lib/misc/dnsmasq.leases" // Default path to the dnsmasq.leases file
const envVarLeasePath = "DNSMASQ_LEASES"                    // Environment variable name for the lease file path

func main() {
	// Determine the lease file path
	leaseFilePath := os.Getenv(envVarLeasePath)
	if leaseFilePath == "" {
		leaseFilePath = defaultLeaseFilePath
		log.Printf("Info: Environment variable %s not set, using default path: %s", envVarLeasePath, defaultLeaseFilePath)
	} else {
		log.Printf("Info: Using lease file path from environment variable %s: %s", envVarLeasePath, leaseFilePath)
	}

	// Open the lease file
	file, err := os.Open(leaseFilePath)
	if err != nil {
		// If the file is not found or permissions are denied, log the error and exit
		log.Fatalf("Error opening file %s: %v", leaseFilePath, err)
	}
	// Ensure the file is closed when the main function exits
	defer file.Close()

	var leases []LeaseEntry // Slice to store the parsed lease entries

	scanner := bufio.NewScanner(file) // Create a scanner to read the file line by line
	lineNumber := 0

	// Read the file line by line
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		fields := strings.Fields(line) // Split the line by whitespace

		// Each valid line should contain 5 fields
		if len(fields) != 5 {
			log.Printf("Warning: Skipping line %d: Invalid number of fields (%d), expected 5. Line: '%s'", lineNumber, len(fields), line)
			continue // Skip malformed line
		}

		// Parse the Unix timestamp (first field)
		expiryTimestampUnix, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			log.Printf("Warning: Skipping line %d: Error parsing timestamp '%s': %v", lineNumber, fields[0], err)
			continue // Skip line with invalid timestamp format
		}

		// Convert Unix timestamp (seconds) to time.Time
		expiryTime := time.Unix(expiryTimestampUnix, 0)

		// Create a LeaseEntry record
		lease := LeaseEntry{
			ExpiryTime:  expiryTime,
			MACAddress:  fields[1],
			IPAddress:   fields[2],
			Hostname:    fields[3],
			ClientID:    fields[4],
		}
		leases = append(leases, lease) // Add the parsed record to the slice
	}

	// Check for errors encountered during scanning
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file %s: %v", leaseFilePath, err)
	}

	// If no leases were found, print a message and exit
	if len(leases) == 0 {
		fmt.Println("No lease entries found or file is empty.")
		return
	}

	// --- Print the table ---
	// Use tabwriter for nicely formatted columns
	// Parameters: output io.Writer, minwidth, tabwidth, padding, padchar, flags
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	// Print table header
	// Use \t as a column separator for tabwriter
	fmt.Fprintln(writer, "Expiry Time\tMAC Address\tIP Address\tHostname\tClient ID")
	fmt.Fprintln(writer, "-----------\t-----------\t----------\t--------\t---------")

	// Print each lease entry
	for _, lease := range leases {
		// Format the time into a readable string (YYYY-MM-DD HH:MM:SS)
		// The reference time `2006-01-02 15:04:05` is Go's standard way to define formats.
		formattedTime := lease.ExpiryTime.Format("2006-01-02 15:04:05")

		// Print the table row
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
			formattedTime,
			lease.MACAddress,
			lease.IPAddress,
			lease.Hostname,
			lease.ClientID,
		)
	}

	// Flush the tabwriter buffer, writing the formatted table to stdout
	writer.Flush()
}
