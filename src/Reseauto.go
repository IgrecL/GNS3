package main

import (
	"Reseauto/src/utils"
	"fmt"
	"io/ioutil"
	//"os"
	"strings"
	"sync"
    "time"

    telnet "github.com/aprice/telnet"
)

func giveIP(ASList []utils.AS, global []utils.Link, ipRange [2]utils.IP) {
	ipMin, ipMax := ipRange[0], ipRange[1]
	ipMin.Mask = 127
	for _, AS := range ASList {
		for i := 0; i < len(AS.RoutersId); i++ {
			for j := 0; j < i; j++ {
				if AS.Adj[i][j] != nil {
					for k := 0; k < 2; k++ {
						if ipMin.Equals(ipMax.Increment()) {
							fmt.Println("L'IP maximale a été atteinte !")
							return
						}
						(AS.Adj[i][j])[k].Ip = ipMin
						fmt.Println((AS.Adj[i][j])[k].Ip.ToString(true))
						ipMin = ipMin.Increment()
					}
				}
			}
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

func generateOutput(ASList []utils.AS, as utils.AS, index int, global []utils.Link, telnetIPs []struct{ID int; IP string}, input string, wg *sync.WaitGroup) {
	var err string
	patterns := [9]string{"routerId", "loopbackAddress", "IGP", "interfaces", "ASN", "BGPRouterId", "neighbors", "neighborsActivate", "redistributeIGP"}
	var replacements [9]string

	routerId := as.RoutersId[index]

	// On récupère la liste des liens eBGP qui concernent le route actuel
	var eBGPNeighbors []utils.Link
	for _, l := range global {
		if l[0].RouterId == routerId || l[1].RouterId == routerId {
			eBGPNeighbors = append(eBGPNeighbors, l)
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
		if len(eBGPNeighbors) != 0 {
			replacements[7] = "\t\t\tredistribute rip ripng\n"
		}
	} else if as.IGP == "OSPF" {
		stringIGP = "ipv6 ospf 1 area 0"
		replacements[8] = "ipv6 router ospf 1\n\t\trouter-id " + replacements[5]
		if len(eBGPNeighbors) != 0 {
			for _, neighbor := range eBGPNeighbors {
				nameId := utils.ToByte(neighbor[0].RouterId != routerId)
				replacements[8] += "\n\t\tpassive-interface " + neighbor[nameId].Name
			}
			replacements[7] = "\t\t\tredistribute ospf 1\n"
		}
	} else {
		err = "Un des protocoles indiqués n'est pas valide !"
	}
	replacements[2] = stringIGP

	// On navigue la matrice d'adjacence pour récupérer les interfaces du routeur
	for _, l := range as.Adj[index] {
		if l != nil {
			nameId := utils.ToByte(l[0].RouterId != routerId)
			tmp := "\tinterface {interface}\n\t\tipv6 enable\n\t\tipv6 address {address}\n\t\t{IGP}\n\t\tno shutdown\n\t\texit\n"
			tmp = utils.RegReplace(tmp, "interface", l[nameId].Name)
			tmp = utils.RegReplace(tmp, "address", l[nameId].Ip.ToString(true))
			replacements[3] += utils.RegReplace(tmp, "IGP", stringIGP)
		}
	}

	for _, id := range as.RoutersId {
		if id != routerId {
			IP := fmt.Sprint(id) + "::" + fmt.Sprint(id)
			replacements[6] += "\t\tneighbor " + IP + " remote-as " + fmt.Sprint(as.ASN) + "\n"
			replacements[6] += "\t\tneighbor " + IP + " update-source Loopback0\n"
			replacements[7] += "\t\t\tneighbor " + IP + " activate\n"
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
		for _, a := range ASList {
			for _, r := range a.RoutersId {
				if r == l[1-nameId].RouterId {
					replacements[6] += "\t\tneighbor " + fmt.Sprint(l[1-nameId].Ip.ToString(false)) + " remote-as " + fmt.Sprint(a.ASN) + "\n"
					replacements[7] += "\t\t\tneighbor " + fmt.Sprint(l[1-nameId].Ip.ToString(false)) + " activate\n"
					break out
				}
			}
		}
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

        time.Sleep(10 * time.Millisecond)
    }

    fmt.Println(" > Finished R" + fmt.Sprint(routerId) + " " + fmt.Sprint(count) + "/" + fmt.Sprint(len([]byte(to_send))) + " bytes sent")

    telnetClient.Close()

	wg.Done()
}

func main() {

	/*if _, err := os.Stat("../out"); os.IsNotExist(err) {
		os.Mkdir("../out", 0700)
	}*/

	var wg sync.WaitGroup

	// On importe le contenu des fichiers .json de AS
	var ASList []utils.AS
	ASList = append(ASList, utils.ImportAS("../intent/AS1.json"))
	ASList = append(ASList, utils.ImportAS("../intent/AS2.json"))

	// On assigne les ASN aux AS
	for i := 0; i < len(ASList); i++ {
		ASList[i].ASN = i + 1
	}

	// On import les liens eBGP des ASBR
	global, ipRange := utils.ImportGlobal("../intent/Global.json")

	// On importe les adresses administratives
	adminInterfaces := utils.ImportAdmin("../intent/Admin.json")
	fmt.Println(adminInterfaces)

	// On attribue les adresses IP parmi celles du range de Global.json
	giveIP(ASList, global, ipRange)

	// On importe la template
	templateByte, err := ioutil.ReadFile("../template/template.ios")
	if err != nil {
		fmt.Println(err)
		return
	}
	template := string(templateByte)

	// On remplace les patterns du template
	for i := range ASList {
		for j := range ASList[i].RoutersId {
			wg.Add(1)
			go generateOutput(ASList, ASList[i], j, global, adminInterfaces, template, &wg)
		}
	}
	wg.Wait()
	fmt.Println("Done.")
}
