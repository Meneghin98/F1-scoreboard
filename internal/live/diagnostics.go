package live

import (
	"net"
	"slices"
	"time"
)

const (
	// Un formato UDP non supportato si segnala nella pagina Live finché continua ad arrivare.
	badFormatFor = 30 * time.Second
	// Gli indirizzi di rete del server si rileggono ogni tanto (cambio di rete, DHCP).
	addressesEvery = time.Minute
)

// TelemetryStatus dice se il server riceve la telemetria e, se no, cosa
// controllare: la pagina Live lo mostra quando non c'è una sessione.
type TelemetryStatus struct {
	Port      int      `json:"port"`      // porta UDP in ascolto, 0 se non è aperta
	Error     string   `json:"error"`     // perché la porta non si apre (per esempio è già in uso)
	Addresses []string `json:"addresses"` // indirizzi IP del server da impostare nel gioco, il principale per primo
	BadFormat uint16   `json:"badFormat"` // formato UDP non supportato arrivato da poco, 0 se nessuno
}

// diagnostics è lo stato della ricezione, protetto da Service.mu.
type diagnostics struct {
	port        int
	err         string
	badFormat   uint16
	badFormatAt time.Time
	addresses   []string
	addressesAt time.Time
}

func (d *diagnostics) status(now time.Time) *TelemetryStatus {
	if now.Sub(d.addressesAt) > addressesEvery {
		d.addresses, d.addressesAt = localAddresses(), now
	}
	st := &TelemetryStatus{Port: d.port, Error: d.err, Addresses: d.addresses}
	if now.Sub(d.badFormatAt) < badFormatFor {
		st.BadFormat = d.badFormat
	}
	return st
}

// localAddresses restituisce gli indirizzi IPv4 della rete locale di questo
// computer. Per primo quello usato per uscire verso internet, che di solito
// è anche quello della rete di casa (gli altri possono essere adattatori virtuali).
func localAddresses() []string {
	addrs := []string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return addrs
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		list, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range list {
			if n, ok := a.(*net.IPNet); ok {
				if ip := n.IP.To4(); ip != nil && ip.IsPrivate() {
					addrs = append(addrs, ip.String())
				}
			}
		}
	}
	// Con UDP Dial non invia nulla: chiede solo al sistema quale indirizzo userebbe.
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		if local, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			if i := slices.Index(addrs, local.IP.String()); i > 0 {
				addrs[0], addrs[i] = addrs[i], addrs[0]
			}
		}
		conn.Close()
	}
	return addrs
}
