package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type IP struct {
	digits [8]int // TODO: uint16
	mask   int
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func toByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func (subnetwork IP) getRange() (IP, IP) {
	mask := 128 - subnetwork.mask
	lowIP := subnetwork
	var highIP IP
	var maskbits [8]uint16
	for i := len(maskbits) - 1; i >= 0; i-- {
		for j := 0; j < min(mask-16*(7-i), 16); j++ {
			maskbits[i] += (1 << j)
		}
	}

	for i := 0; i < len(maskbits); i++ {
		highIP.digits[i] = subnetwork.digits[i] | int(maskbits[i])
	}

	highIP.mask = subnetwork.mask

	return lowIP, highIP
}

func (ip IP) toString(withMask bool) string {
	var str string
	for i := 0; i < 8; i++ {
		str += strconv.FormatInt(int64(ip.digits[i]), 16) + ":"
	}
	str = str[:len(str)-1]
	if withMask {
		str += "/" + fmt.Sprint(ip.mask)
	}
	return str
}

func (ip *IP) toInt(ipString string) {
	split := strings.Split(ipString, ":")
	if len(split) < 8 {
		var splitComplet [8]string
		var stop bool
		var i, j int = 0, 0
		for ; !stop; i++ {
			splitComplet[i] = split[i]
			if split[i] == "" {
				stop = true
			}
		}
		stop = false
		for ; !stop; j++ {
			splitComplet[7-j] = split[len(split)-1-j]
			if split[len(split)-1-j] == "" {
				stop = true
			}
		}
		split = splitComplet[:]
	}
	split2 := strings.Split(split[7], "/")
	split[7] = split2[0]
	ip.mask, _ = strconv.Atoi(split2[1])
	for i, s := range split {
		numHex, _ := strconv.ParseInt(s, 16, 64)
		ip.digits[i] = int(numHex)
	}
}

func (ip IP) increment() IP {
	var new IP
	for i, v := range ip.digits {
		new.digits[i] = v
	}
	new.mask = ip.mask
	for i := 7; i >= 0; i-- {
		if new.digits[i] == 65535 {
			new.digits[i] = 0
		} else {
			new.digits[i]++
			break
		}
	}
	return new
}

func (ip1 IP) equals(ip2 IP) bool {
	for i := 0; i < 8; i++ {
		if ip1.digits[i] != ip2.digits[i] {
			return false
		}
	}
	return true
}

type Interface struct {
	name     string
	ip       IP
	routerId int
	ASBR     bool
}

type Link [2]Interface

type AS struct {
	ASN       int
	IGP       string
	routersId []int
	adj       [][]*Link
}

func importGlobal(url string) ([]Link, [2]IP) {
	// Importing .json files
	file, _ := os.Open(url)
	defer file.Close()

	// Lecture du .json
	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		var nilArray [2]IP
		return nil, nilArray
	}

	// On caste le contenu en map de string
	var linksMap map[string]interface{}
	err = json.Unmarshal([]byte(data), &linksMap)
	if err != nil {
		fmt.Println(err)
		var nilArray [2]IP
		return nil, nilArray
	}

	// On récupère la pool d'IP
	var ipRange [2]IP
	ipRange[0].toInt(linksMap["ip_range"].([]any)[0].(string))
	ipRange[1].toInt(linksMap["ip_range"].([]any)[1].(string))

	// On boucle dans la map pour extraire les valeurs et créer un []Link
	var links []Link
	for _, value := range linksMap["links"].([]any) {
		var link Link
		for i, value2 := range value.([]any) {
			var in Interface
			in.routerId = int(value2.(map[string]any)["id"].(float64))
			in.name = value2.(map[string]any)["interface"].(string)
			in.ASBR = true
			link[i] = in
		}
		links = append(links, link)
	}

	return links, ipRange
}

func importAS(url string) AS {
	// Importing .json files
	file, _ := os.Open(url)
	defer file.Close()

	// Lecture du .json
	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		return *new(AS)
	}

	// On caste le contenu en map de string
	var ASMap map[string]any
	err = json.Unmarshal([]byte(data), &ASMap)
	if err != nil {
		fmt.Println(err)
		return *new(AS)
	}

	// On importe le protocole IGP utilisé
	var as AS
	as.IGP = ASMap["protocol"].(string)

	// On importe la liste des routeurs de l'AS
	for _, value := range ASMap["routers"].([]any) {
		as.routersId = append(as.routersId, int(value.(float64)))
	}

	// On initialise la matrice d'adjacence
	as.adj = make([][]*Link, len(as.routersId))
	for i := range as.routersId {
		as.adj[i] = make([]*Link, len(as.routersId))
	}

	// On remplit la matrice d'adjacence
	for _, value := range ASMap["links"].([]any) {

		// On récupère les id et les noms d'interface
		var int1, int2 Interface
		int1.routerId = int(value.([]any)[0].(map[string]any)["id"].(float64))
		int2.routerId = int(value.([]any)[1].(map[string]any)["id"].(float64))
		int1.name = value.([]any)[0].(map[string]any)["interface"].(string)
		int2.name = value.([]any)[1].(map[string]any)["interface"].(string)
		var link Link
		link[0], link[1] = int1, int2

		// On récupère l'indice du routeur dans la liste de routeurs
		var index1, index2 int
		for i, id := range as.routersId {
			if id == int1.routerId {
				index1 = i
			}
			if id == int2.routerId {
				index2 = i
			}
		}

		// On remplit la matrice d'adjacence de liens
		as.adj[index1][index2] = &link
		as.adj[index2][index1] = &link
	}

	return as
}

func printMat(M [][]int) {
	for _, v := range M {
		for _, w := range v {
			fmt.Print(w, " ")
		}
		fmt.Println()
	}
}

func giveIP(ASList []AS, global []Link, ipRange [2]IP) {
	ipMin, ipMax := ipRange[0], ipRange[1]
	for _, AS := range ASList {
		for i := 0; i < len(AS.routersId); i++ {
			for j := 0; j < i; j++ {
				if AS.adj[i][j] != nil {
					for k := 0; k < 2; k++ {
						if ipMin.equals(ipMax.increment()) {
							fmt.Println("L'IP maximale a été atteinte !")
							return
						}
						(AS.adj[i][j])[k].ip = ipMin
						fmt.Println((AS.adj[i][j])[k].ip.toString(true))
						ipMin = ipMin.increment()
					}
				}
			}
		}
	}
	for i := 0; i < len(global); i++ {
		for k := 0; k < 2; k++ {
			if ipMin.equals(ipMax.increment()) {
				fmt.Println("L'IP maximale a été atteinte !")
				return
			}
			(global[i])[k].ip = ipMin
			fmt.Println((global[i])[k].ip.toString(true))
			ipMin = ipMin.increment()
		}
	}
}

func regReplace(input, regex, text string) string {
	regexp, err := regexp.Compile("{" + regex + "}")
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return regexp.ReplaceAllString(input, text)
}

func generateOutput(ASList []AS, as AS, index int, global []Link, input string, wg *sync.WaitGroup) {
	var err string
	patterns := [9]string{"routerId", "loopbackAddress", "IGP", "interfaces", "ASN", "BGPRouterId", "neighbors", "neighborsActivate", "redistributeIGP"}
	var replacements [9]string

	routerId := as.routersId[index]

	// On récupère la liste des liens eBGP qui concernent le route actuel
	var eBGPNeighbors []Link
	for _, l := range global {
		if l[0].routerId == routerId || l[1].routerId == routerId {
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
		replacements[8] = "ipv6 router rip ripng\n redistribute connected"
		if len(eBGPNeighbors) != 0 {
			replacements[7] = "  redistribute rip ripng\n"
		}
	} else if as.IGP == "OSPF" {
		stringIGP = "ipv6 ospf 1 area 0"
		replacements[8] = "ipv6 router ospf 1\n router-id " + replacements[5]
		if len(eBGPNeighbors) != 0 {
			replacements[7] = "  redistribute ospf 1\n"
		}
	} else {
		err = "Un des protocoles indiqués n'est pas valide !"
	}
	replacements[2] = stringIGP

	// On navigue la matrice d'adjacence pour récupérer les interfaces du routeur
	for _, l := range as.adj[index] {
		if l != nil {
			nameId := toByte(l[0].routerId != routerId)
			tmp := "interface {interface}\n no ip address\n speed auto\n duplex auto\n ipv6 address {address}\n ipv6 enable\n {IGP}\n!\n"
			tmp = regReplace(tmp, "interface", l[nameId].name)
			tmp = regReplace(tmp, "address", l[nameId].ip.toString(true))
			replacements[3] += regReplace(tmp, "IGP", stringIGP)
		}
	}

	for _, id := range as.routersId {
		if id != routerId {
			IP := fmt.Sprint(id) + "::" + fmt.Sprint(id)
			replacements[6] += " neighbor " + IP + " remote-as " + fmt.Sprint(as.ASN) + "\n"
			replacements[6] += " neighbor " + IP + " update-source Loopback0\n"
			replacements[7] += "  neighbor " + IP + " activate\n"
		}
	}

	for _, l := range eBGPNeighbors {
		nameId := toByte(l[0].routerId != routerId)
		tmp := "interface {interface}\n no ip address\n speed auto\n duplex auto\n ipv6 address {address}\n ipv6 enable\n!\n"
		tmp = regReplace(tmp, "interface", l[nameId].name)
		replacements[3] += regReplace(tmp, "address", l[nameId].ip.toString(true))
	out:
		for _, a := range ASList {
			for _, r := range a.routersId {
				if r == l[1-nameId].routerId {
					replacements[6] += " neighbor " + fmt.Sprint(l[1-nameId].ip.toString(false)) + " remote-as " + fmt.Sprint(a.ASN) + "\n"
					replacements[7] += "  neighbor " + fmt.Sprint(l[1-nameId].ip.toString(false)) + " activate\n"
					break out
				}
			}
		}
	}

	replacements[3] = strings.Trim(replacements[3], "\n")
	replacements[6] = strings.Trim(replacements[6], "\n")
	replacements[7] = strings.Trim(replacements[7], "\n")

	for i, p := range patterns {
		input = regReplace(input, p, replacements[i])
	}

	fmt.Println(err)
	if err2 := os.WriteFile(fmt.Sprint(index)+".cfg", []byte(input), 0666); err2 != nil {
		fmt.Println(err2)
		return
	}

	wg.Done()
}

func main() {

	var wg sync.WaitGroup

	// On importe le contenu des fichiers .json de AS
	var ASList []AS
	ASList = append(ASList, importAS("AS1.json"))
	ASList = append(ASList, importAS("AS2.json"))

	// On assigne les ASN aux AS
	for i := 0; i < len(ASList); i++ {
		ASList[i].ASN = i
	}

	// On import les liens eBGP des ASBR
	global, ipRange := importGlobal("Global.json")

	// On attribue les adresses IP parmi celles du range de Global.json
	giveIP(ASList, global, ipRange)

	// fmt.Println(ASList[0].adj[0])
	// for _, a := range ASList[0].adj {
	// 	for _, i := range a {
	// 		if i != nil {
	// 			fmt.Println(i[0].ip.toString(true))
	// 		}
	// 	}
	// }

	// On importe la template
	templateByte, err := ioutil.ReadFile("template.cfg")
	if err != nil {
		fmt.Println(err)
		return
	}
	template := string(templateByte)

	// On remplace les patterns du template
	for i := range ASList {
		for j := range ASList[i].routersId {
			wg.Add(1)
			go generateOutput(ASList, ASList[i], j, global, template, &wg)
		}
	}
	wg.Wait()
	fmt.Println("Done.")
	// mettre routeur id

	global, ipRange, ASList = global, ipRange, ASList
}
