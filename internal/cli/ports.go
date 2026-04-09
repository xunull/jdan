package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/ports"
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "显示本机正在监听的网络端口",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		tcpOnly, _ := cmd.Flags().GetBool("tcp")
		udpOnly, _ := cmd.Flags().GetBool("udp")

		tcp := !udpOnly
		udp := !tcpOnly

		entries, err := ports.CollectPorts(tcp, udp)
		if err != nil {
			return err
		}

		if jsonOut {
			out, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		}

		return printTable(entries)
	},
}

func printTable(entries []ports.PortEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PROTOCOL\tADDRESS\tPORT\tPROCESS")

	var tcpEntries, udpEntries []ports.PortEntry
	for _, e := range entries {
		if e.Protocol == "TCP" {
			tcpEntries = append(tcpEntries, e)
		} else {
			udpEntries = append(udpEntries, e)
		}
	}

	for _, e := range tcpEntries {
		fmt.Fprintf(w, "TCP\t%s\t%d\t%s\n", e.Address, e.Port, e.Process)
	}
	if len(udpEntries) > 0 && len(tcpEntries) > 0 {
		fmt.Fprintln(w)
	}
	for _, e := range udpEntries {
		fmt.Fprintf(w, "UDP\t%s\t%d\t%s\n", e.Address, e.Port, e.Process)
	}

	return w.Flush()
}

func init() {
	portsCmd.Flags().BoolP("json", "j", false, "以 JSON 格式输出")
	portsCmd.Flags().BoolP("tcp", "t", false, "仅显示 TCP 端口")
	portsCmd.Flags().BoolP("udp", "u", false, "仅显示 UDP 端口")
	rootCmd.AddCommand(portsCmd)
}
