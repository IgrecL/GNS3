package main

import (
	"Reseauto/src/utils"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"
	"flag"
    "math"

	telnet "github.com/aprice/telnet"
)

func giveIP(ASList []utils.AS, global []utils.Link, ipRange [2]utils.IP) {
	ipMin, ipMax := ipRange[0], ipRange[1]
	ipMin.Mask = 127

	for _, AS := range ASList {
        var addressed = 0
        var ipMinInAS, ipMaxInAS utils.IP
        ipMinInAS = ipMin
		for i := 0; i < len(AS.RoutersId); i++ {
			for j := 0; j < i; j++ {
				if AS.Adj[i][j] != nil {
					for k := 0; k < 2; k++ {
						if ipMin.Equals(ipMax.Increment()) {
							fmt.Println("L'IP maximale a été atteinte !")
							return
						}
						(AS.Adj[i][j])[k].Ip = ipMin
                        ipMaxInAS = ipMin
                        addressed++
						fmt.Println((AS.Adj[i][j])[k].Ip.ToString(true))
						ipMin = ipMin.Increment()
					}
				}
			}
		}

        minimalMaskForAS := float64(utils.MaskForIPRange(ipMinInAS, ipMaxInAS).Mask)
        fmt.Println("/" + fmt.Sprint(minimalMaskForAS))
        rest := int(math.Pow(2, 128 - minimalMaskForAS)) - addressed
        for i := 0; i < rest; i++ {
            ipMin = ipMin.Increment()
        }
	}
	for i := 0; i < len(global); i++ {
		for k := 0; k < 2; k++ {
			if ipMin.Equals(ipMax.Increment()) {
				fmt.Println("L'IP maximale a été atteinte !")
				return
			}
			(global[i])[k].Ip = ipMin
			fmt.Println((global[i])[k].Ip.ToString(true))
			ipMin = ipMin.Increment()
		}
	}
}

func generateConfs(ASList []utils.AS, as utils.AS, index int, global []utils.Link, meds [][2]int, input string, wg *sync.WaitGroup) {
	var err string
	patterns := [13]string{"routerId", "loopbackAddress", "IGP", "interfaces", "ASN", "BGPRouterId", "neighbors", "neighborsActivate", "redistributeIGP", "routeMaps", "aggregate", "blackholeRoute", "communities"}
	var replacements [13]string

	routerId := as.RoutersId[index]

	// On récupère la liste des liens eBGP qui concernent le route actuel
	var eBGPNeighbors []utils.Link
	for _, l := range global {
		if l[0].RouterId == routerId || l[1].RouterId == routerId {
			eBGPNeighbors = append(eBGPNeighbors, l)
		}
	}

    routeMapsOut := make(map[utils.IP]([]string))
    routeMapsIn := make(map[utils.IP]([]string))

    type MedReplacement struct {
        ip utils.IP
        med int
    }
    var medsReplacements []MedReplacement
    for i, l := range global {
        if l[0].RouterId == routerId {
            medReplacement := MedReplacement{ip: l[1].Ip, med: meds[i][0]}
            medsReplacements = append(medsReplacements, medReplacement)
        } else if l[1].RouterId == routerId {
            medReplacement := MedReplacement{ip: l[0].Ip, med: meds[i][1]}
            medsReplacements = append(medsReplacements, medReplacement)
        }
    }

    for _, m := range medsReplacements {
        entries, ok := routeMapsOut[m.ip]
        if ok {
            entries = append(entries, " set metric " + fmt.Sprint(m.med) + "\n")
        } else {
            routeMapsOut[m.ip] = []string{" set metric " + fmt.Sprint(m.med) + "\n"}
        }
    }

	replacements[0] = "R" + fmt.Sprint(routerId)
	replacements[1] = fmt.Sprint(routerId) + "::" + fmt.Sprint(routerId)
	replacements[4] = fmt.Sprint(as.ASN)
	replacements[5] = fmt.Sprint(routerId) + "." + fmt.Sprint(routerId) + "." + fmt.Sprint(routerId) + "." + fmt.Sprint(routerId)

	// Portion de ligne qui spécifie le protocole utilisé en IGP
	var stringIGP string
	if as.IGP == "RIP" {
		stringIGP = "ipv6 rip ripng enable"
		replacements[8] = "ipv6 router rip ripng\n redistribute connected"
	} else if as.IGP == "OSPF" {
		stringIGP = "ipv6 ospf 1 area 0"
		replacements[8] = "ipv6 router ospf 1\n router-id " + replacements[5]
		if len(eBGPNeighbors) != 0 {
			for _, neighbor := range eBGPNeighbors {
				nameId := utils.ToByte(neighbor[0].RouterId != routerId)
				replacements[8] += "\n passive-interface " + neighbor[nameId].Name
			}
		}
	} else {
		err = "Un des protocoles indiqués n'est pas valide !"
	}
	replacements[2] = stringIGP

	// On navigue la matrice d'adjacence pour récupérer les interfaces du routeur
	for _, l := range as.Adj[index] {
		if l != nil {
			nameId := utils.ToByte(l[0].RouterId != routerId)
			tmp := "interface {interface}\n no ip address\n speed auto\n duplex auto\n ipv6 address {address}\n ipv6 enable\n {IGP}\n!\n"
			tmp = utils.RegReplace(tmp, "interface", l[nameId].Name)
			tmp = utils.RegReplace(tmp, "address", l[nameId].Ip.ToString(true))
			replacements[3] += utils.RegReplace(tmp, "IGP", stringIGP)
		}
	}

	for neighborIndex, id := range as.RoutersId {
		if id != routerId {
			IP := fmt.Sprint(id) + "::" + fmt.Sprint(id)
			replacements[6] += " neighbor " + IP + " remote-as " + fmt.Sprint(as.ASN) + "\n"
			replacements[6] += " neighbor " + IP + " update-source Loopback0\n"
			replacements[7] += "  neighbor " + IP + " activate\n"
			replacements[7] += "  neighbor " + IP + " send-community\n"
			pref := as.LocalPrefs[index][neighborIndex]
			if pref != 0 {
                var ip utils.IP
                ip.ToInt(IP + "/0")
                entry := " set local-preference "+ fmt.Sprint(pref) + "\n"
                entries, ok := routeMapsIn[ip]
                if ok {
                    routeMapsIn[ip] = append(entries, entry)
                } else {
                    routeMapsIn[ip] = []string{entry}
                }
            }
        }
	}

	for _, l := range eBGPNeighbors {
		nameId := utils.ToByte(l[0].RouterId != routerId)
		tmp := "interface {interface}\n no ip address\n speed auto\n duplex auto\n ipv6 address {address}\n ipv6 enable\n {IGP}\n"
		if as.IGP == "OSPF" {
			tmp = utils.RegReplace(tmp, "IGP", stringIGP)
		} else {
			tmp = utils.RegReplace(tmp, "IGP", "")
			tmp = tmp[:len(tmp)-2]
		}
		tmp += "!\n"
		tmp = utils.RegReplace(tmp, "interface", l[nameId].Name)
		replacements[3] += utils.RegReplace(tmp, "address", l[nameId].Ip.ToString(true))
	out:
		for i, a := range ASList {
			for _, r := range a.RoutersId {
				if r == l[1-nameId].RouterId {
					IP := fmt.Sprint(l[1-nameId].Ip.ToString(false))
					replacements[6] += " neighbor " + IP + " remote-as " + fmt.Sprint(a.ASN) + "\n"
					replacements[7] += "  neighbor " + IP + " activate\n"
					prepend := as.Prepends[i]
					if prepend != 0 {
                        entry := " set as-path prepend "
                        for x := 0; x < prepend; x++ {
                            entry += fmt.Sprint(as.ASN) + " "   
                        }
                        entry += "\n"
                        entries, ok := routeMapsOut[l[1-nameId].Ip]
                        if ok {
                            routeMapsOut[l[1-nameId].Ip] = append(entries, entry)
                        } else {
                            routeMapsOut[l[1-nameId].Ip] = []string{entry}
                        }
                    }
                    community := as.Communities[i]
                    var entry string
                    if community == "Peer" {
                        entry = " set community 1\n"
                    } else if community == "Client" {
                        entry = " set community 2\n"
                    } else if community == "Provider" {
                        entry = " set community 3\n"
                    }
                    entries, ok := routeMapsIn[l[1-nameId].Ip]
                    if ok {
                        routeMapsIn[l[1-nameId].Ip] = append(entries, entry)
                    } else {
                        routeMapsIn[l[1-nameId].Ip] = []string{entry}
                    }
                    break out
                }
			}
		}
	}

    minIP, maxIP := utils.MaxIP(), utils.MinIP()
    for _, iv := range as.Adj {
        for _, l := range iv {
            if l != nil {
                for _, interfac := range l {
                    if interfac.Ip.GreaterThan(maxIP) {
                        maxIP = interfac.Ip
                    }

                    if interfac.Ip.LessThan(minIP) {
                        minIP = interfac.Ip
                    }
                }
            }
        }
    }

    aggregNet := utils.MaskForIPRange(minIP, maxIP)
    if len(eBGPNeighbors) != 0 {
        replacements[10]  = "  network " + aggregNet.ToString(true) + "\n"
        replacements[11]  = "ipv6 route " + aggregNet.ToString(true) + " null0"
    }

    for k, v := range routeMapsOut {
        IP := k.ToString(false)
        replacements[7] += "  neighbor " + IP + " route-map OUTMAP-" + IP + " out\n"
        replacements[9] += "route-map OUTMAP-" + IP + " permit 10\n"
        for _, s := range v {
            replacements[9] += s
        }
        replacements[9] += "!\n"
    }

    for k, v := range routeMapsIn {
        IP := k.ToString(false)
        replacements[7] += "  neighbor " + IP + " route-map INMAP-" + IP + " in\n"
        replacements[9] += "route-map INMAP-"+ IP + " permit 10\n"
        for _, s := range v {
            replacements[9] += s
        }
        replacements[9] += "!\n"
    }

	replacements[3] = strings.Trim(replacements[3], "\n")
	replacements[6] = strings.Trim(replacements[6], "\n")
	replacements[7] = strings.Trim(replacements[7], "\n")

	for i, p := range patterns {
		input = utils.RegReplace(input, p, replacements[i])
	}

	if err != "" {
		fmt.Println(err)
	}
	if err2 := os.WriteFile("out/i"+fmt.Sprint(routerId)+"_startup-config.cfg", []byte(input), 0666); err2 != nil {
		fmt.Println(err2)
		return
	}

	wg.Done()
}

func generateTelnet(ASList []utils.AS, as utils.AS, index int, global []utils.Link, meds [][2]int, telnetIPs []struct{ID int; IP string}, input string, telnetDelay int, wg *sync.WaitGroup) {
	var err string
	patterns := [11]string{"routerId", "loopbackAddress", "IGP", "interfaces", "ASN", "BGPRouterId", "neighbors", "neighborsActivate", "redistributeIGP", "aggregate", "blackholeRoute"}
	var replacements [11]string

	routerId := as.RoutersId[index]

	// On récupère la liste des liens eBGP qui concernent le route actuel
	var eBGPNeighbors []utils.Link
	for _, l := range global {
		if l[0].RouterId == routerId || l[1].RouterId == routerId {
			eBGPNeighbors = append(eBGPNeighbors, l)
		}
	}

    routeMapsOut := make(map[utils.IP]([]string))
    routeMapsIn := make(map[utils.IP]([]string))

    type MedReplacement struct {
        ip utils.IP
        med int
    }
    var medsReplacements []MedReplacement
    for i, l := range global {
        if l[0].RouterId == routerId {
            medReplacement := MedReplacement{ip: l[1].Ip, med: meds[i][0]}
            medsReplacements = append(medsReplacements, medReplacement)
        } else if l[1].RouterId == routerId {
            medReplacement := MedReplacement{ip: l[0].Ip, med: meds[i][1]}
            medsReplacements = append(medsReplacements, medReplacement)
        }
    }

    for _, m := range medsReplacements {
        entries, ok := routeMapsOut[m.ip]
        if ok {
            entries = append(entries, " set metric " + fmt.Sprint(m.med) + "\n")
        } else {
            routeMapsOut[m.ip] = []string{" set metric " + fmt.Sprint(m.med) + "\n"}
        }
    }

	replacements[0] = "R" + fmt.Sprint(routerId)
	replacements[1] = fmt.Sprint(routerId) + "::" + fmt.Sprint(routerId)
	replacements[4] = fmt.Sprint(as.ASN)
	replacements[5] = fmt.Sprint(routerId) + "." + fmt.Sprint(routerId) + "." + fmt.Sprint(routerId) + "." + fmt.Sprint(routerId)

	// Portion de ligne qui spécifie le protocole utilisé en IGP
	var stringIGP string
	if as.IGP == "RIP" {
		stringIGP = "ipv6 rip ripng enable"
		replacements[8] = "ipv6 router rip ripng\n\t\tredistribute connected"
	} else if as.IGP == "OSPF" {
		stringIGP = "ipv6 ospf 1 area 0"
		replacements[8] = "ipv6 router ospf 1\n\t\trouter-id " + replacements[5]
		if len(eBGPNeighbors) != 0 {
			for _, neighbor := range eBGPNeighbors {
				nameId := utils.ToByte(neighbor[0].RouterId != routerId)
				replacements[8] += "\n\t\tpassive-interface " + neighbor[nameId].Name
			}
		}
	} else {
		err = "Un des protocoles indiqués n'est pas valide !"
	}
	replacements[2] = stringIGP

	// On navigue la matrice d'adjacence pour récupérer les interfaces du routeur
	for _, l := range as.Adj[index] {
		if l != nil {
			nameId := utils.ToByte(l[0].RouterId != routerId)
			tmp := "\tinterface {interface}\n\t\tipv6 enable{OSPFCost}\n\t\tipv6 address {address}\n\t\t{IGP}\n\t\tno shutdown\n\t\texit\n"
			tmp = utils.RegReplace(tmp, "interface", l[nameId].Name)
			tmp = utils.RegReplace(tmp, "address", l[nameId].Ip.ToString(true))
			cost := l[nameId].OSPFCost
			if as.IGP == "OSPF" && cost != -1 {
				tmp = utils.RegReplace(tmp, "OSPFCost", "\n\t\tipv6 ospf cost " + fmt.Sprint(cost))
			} else {
				tmp = utils.RegReplace(tmp, "OSPFCost", "")
			}
			replacements[3] += utils.RegReplace(tmp, "IGP", stringIGP)
		}
	}

	for neighborIndex, id := range as.RoutersId {
		if id != routerId {
			IP := fmt.Sprint(id) + "::" + fmt.Sprint(id)
			replacements[6] += "\t\tneighbor " + IP + " remote-as " + fmt.Sprint(as.ASN) + "\n"
			replacements[6] += "\t\tneighbor " + IP + " update-source Loopback0\n"
			replacements[7] += "\t\t\tneighbor " + IP + " activate\n"
			replacements[7] += "\t\t\tneighbor " + IP + " send-community\n"
			pref := as.LocalPrefs[index][neighborIndex]
			if pref != 0 {
                var ip utils.IP
                ip.ToInt(IP + "/0")
                entry := " set local-preference "+ fmt.Sprint(pref) + "\n"
                entries, ok := routeMapsIn[ip]
                if ok {
                    routeMapsIn[ip] = append(entries, entry)
                } else {
                    routeMapsIn[ip] = []string{entry}
                }
            }
		}
	}

	for _, l := range eBGPNeighbors {
		nameId := utils.ToByte(l[0].RouterId != routerId)
		tmp := "\tinterface {interface}\n\t\tipv6 enable\n\t\tipv6 address {address}\n\t\t{IGP}no shutdown\n\t\texit\n"
		if as.IGP == "OSPF" {
			tmp = utils.RegReplace(tmp, "IGP", stringIGP+"\n\t\t")
		} else {
			tmp = utils.RegReplace(tmp, "IGP", "")
		}
		tmp = utils.RegReplace(tmp, "interface", l[nameId].Name)
		replacements[3] += utils.RegReplace(tmp, "address", l[nameId].Ip.ToString(true))
	out:
		for i, a := range ASList {
			for _, r := range a.RoutersId {
				if r == l[1-nameId].RouterId {
					IP := l[1-nameId].Ip.ToString(false)
					replacements[6] += "\t\tneighbor " + IP + " remote-as " + fmt.Sprint(a.ASN) + "\n"
					replacements[7] += "\t\t\tneighbor " + IP + " activate\n"
					prepend := as.Prepends[i]
                    if prepend != 0 {
                        entry := "\t\t\t\tset as-path prepend "
                        for x := 0; x < prepend; x++ {
                            entry += fmt.Sprint(as.ASN) + " "   
                        }
                        entry += "\n"
                        entries, ok := routeMapsOut[l[1-nameId].Ip]
                        if ok {
                            routeMapsOut[l[1-nameId].Ip] = append(entries, entry)
                        } else {
                            routeMapsOut[l[1-nameId].Ip] = []string{entry}
                        }
                    }
                    community := as.Communities[i]
                    var entry string
                    if community == "Peer" {
                        entry = "\t\t\t\tset community 1\n"
                    } else if community == "Client" {
                        entry = "\t\t\t\tset community 2\n"
                    } else if community == "Provider" {
                        entry = "\t\t\t\tset community 3\n"
                    }
                    entries, ok := routeMapsIn[l[1-nameId].Ip]
                    if ok {
                        routeMapsIn[l[1-nameId].Ip] = append(entries, entry)
                    } else {
                        routeMapsIn[l[1-nameId].Ip] = []string{entry}
                    }
                    break out
                }
			}
		}
	}

    minIP, maxIP := utils.MaxIP(), utils.MinIP()
    for _, iv := range as.Adj {
        for _, l := range iv {
            if l != nil {
                for _, interfac := range l {
                    if interfac.Ip.GreaterThan(maxIP) {
                        maxIP = interfac.Ip
                    }

                    if interfac.Ip.LessThan(minIP) {
                        minIP = interfac.Ip
                    }
                }
            }
        }
    }

    aggregNet := utils.MaskForIPRange(minIP, maxIP)
    if len(eBGPNeighbors) != 0 {
        replacements[9]  = "\t\tnetwork " + aggregNet.ToString(true)
        replacements[10]  = "ipv6 route " + aggregNet.ToString(true) + " null0"
    }

    for k, v := range routeMapsOut {
        IP := k.ToString(false)
        replacements[7] += "\t\t\tneighbor " + IP + " route-map MAP-" + IP + " out\n"
        replacements[7] += "\t\t\t\troute-map MAP-" + IP + " permit 10\n"
        for _, s := range v {
            replacements[7] += s
        }
        replacements[7] += "\t\t\t\texit\nrouter bgp " + replacements[4] + "\naddress-family ipv6 unicast\n"
    }

    for k, v := range routeMapsIn {
        IP := k.ToString(false)
        replacements[7] += "\t\t\tneighbor " + IP + " route-map INMAP-" + IP + " in\n"
        replacements[7] += "\t\t\t\troute-map INMAP-"+ IP + " permit 10\n"
        for _, s := range v {
            replacements[7] += s
        }
        replacements[7] += "\t\t\t\texit\nrouter bgp " + replacements[4] + "\naddress-family ipv6 unicast\n"
    }

	replacements[3] = strings.Trim(replacements[3], "\n")
	replacements[6] = strings.Trim(replacements[6], "\n")
	replacements[7] = strings.Trim(replacements[7], "\n")

	for i, p := range patterns {
		input = utils.RegReplace(input, p, replacements[i])
	}

	if err != "" {
		fmt.Println(err)
	}
	
	/*if err2 := os.WriteFile("../out/R"+fmt.Sprint(routerId)+".ios", []byte(input), 0666); err2 != nil {
		fmt.Println(err2)
		return
	}*/

	// Logique a placer dans le main ?? mais du coup non parallelisee...
	var telnetIP string
	for _, v := range telnetIPs {
		if v.ID == routerId {
			telnetIP = v.IP
		}
	}
	if telnetIP == "" {
		return
	}

	fmt.Println("Connecting to", telnetIP)
	telnetClient, telnetError := telnet.Dial(telnetIP)
	if telnetError != nil {
		fmt.Println("Error occured when connecting to R" + fmt.Sprint(routerId))
		return
	}
	fmt.Println(" > Writing config... R" + fmt.Sprint(routerId))

	to_send := utils.RegCarriage(input)
	//fmt.Println(to_send)

	var count int = 0
	for _, b := range []byte(to_send) {
		written, err2 := telnetClient.Write([]byte{b})
		count += written
		if err2 != nil {
			fmt.Println("Error occured when writing R" + fmt.Sprint(routerId))
		return
		}

		time.Sleep(time.Duration(telnetDelay) * time.Millisecond)
	}

	fmt.Println(" > Finished R" + fmt.Sprint(routerId) + " " + fmt.Sprint(count) + "/" + fmt.Sprint(len([]byte(to_send))) + " bytes sent")

	telnetClient.Close()

	wg.Done()
}

func main() {
	var wg sync.WaitGroup

	// On importe le contenu des fichiers .json de AS
    ASList := utils.ImportAS("./intent/")

    // On import les liens eBGP des ASBR
	global, meds, ipRange := utils.ImportGlobal("./intent/Global.json", ASList)

    fmt.Println("==========")
    fmt.Println(meds)
    fmt.Println("==========")

	// On importe les adresses administratives
	adminInterfaces := utils.ImportAdmin("./intent/Admin.json")
	fmt.Println(adminInterfaces)

	// On attribue les adresses IP parmi celles du range de Global.json
	giveIP(ASList, global, ipRange)

	

	modePtr := flag.String("mode", "config", "Specify output mode: config | telnet")
	delayPtr := flag.Int("delay", 10, "Specify telnet delay")

	flag.Parse()

	if *modePtr == "telnet" {
		telnetDelay := *delayPtr
		fmt.Println("Delay for telnet communication set to:", telnetDelay, "ms")

		// On importe la template
		templateByte, err := ioutil.ReadFile("./template/template.ios")
		if err != nil {
			fmt.Println(err)
			return
		}

		template := string(templateByte)

		// On remplace les patterns du template
		for i := range ASList {
			for j := range ASList[i].RoutersId {
				wg.Add(1)
				go generateTelnet(ASList, ASList[i], j, global, meds, adminInterfaces, template, telnetDelay, &wg)
			}
		}
	} else if *modePtr == "config" {
		if _, err := os.Stat("./out"); os.IsNotExist(err) {
			os.Mkdir("./out", 0700)
		}
		// On importe la template
		templateByte, err := ioutil.ReadFile("./template/template.cfg")
		if err != nil {
			fmt.Println(err)
			return
		}

		template := string(templateByte)

		// On remplace les patterns du template
		for i := range ASList {
			for j := range ASList[i].RoutersId {
				wg.Add(1)
				go generateConfs(ASList, ASList[i], j, global, meds, template, &wg)
			}
		}
	} else {
		fmt.Println("Error, the '-mode' flag can be set to either 'config' or 'telnet'")
		return
	}

	wg.Wait()
	fmt.Println("Done.")
}
