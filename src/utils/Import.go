package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func ImportGlobal(url string, ASList []AS) ([]Link, [2]IP) {
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
	var subnet IP
	subnet.ToInt(linksMap["ip_range"].([]any)[0].(string))
	var ipRange [2]IP
	ipRange[0], ipRange[1] = subnet.GetRange()

	// On boucle dans la map pour extraire les valeurs et créer un []Link
	var links []Link
	for _, value := range linksMap["links"].([]any) {
		var link Link
		for i, value2 := range value.([]any) {
			var in Interface
			in.RouterId = int(value2.(map[string]any)["id"].(float64))
			in.Name = value2.(map[string]any)["interface"].(string)
			in.ASBR = true
			link[i] = in
		}
		links = append(links, link)
	}

	for i, _ := range ASList {
		ASList[i].Prepends = make([]int, len(ASList))
	}

	for _, value := range linksMap["as_prepends"].([]any) {
		from := int(value.(map[string]any)["from"].(float64))
		to := int(value.(map[string]any)["to"].(float64))
		times := int(value.(map[string]any)["times"].(float64))
		
		index1, index2 := -1, -1
		for index, a := range ASList {
			if a.ASN == from {
				index1 = index
			} else if a.ASN == to {
				index2 = index
			}
		}
		if index1 == -1 || index2 == -1 {
			fmt.Println("Error while loading prependings")
			break
		}

		ASList[index1].Prepends[index2] = times

	}

	return links, ipRange
}

func ImportAS(url string) AS {
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
		as.RoutersId = append(as.RoutersId, int(value.(float64)))
	}

	// On initialise la matrice d'adjacence
	as.Adj = make([][]*Link, len(as.RoutersId))
	for i := range as.RoutersId {
		as.Adj[i] = make([]*Link, len(as.RoutersId))
	}

	// On remplit la matrice d'adjacence
	for _, value := range ASMap["links"].([]any) {

		// On récupère les id et les noms d'interface
		var int1, int2 Interface
		int1.RouterId = int(value.([]any)[0].(map[string]any)["id"].(float64))
		int2.RouterId = int(value.([]any)[1].(map[string]any)["id"].(float64))
		int1.Name = value.([]any)[0].(map[string]any)["interface"].(string)
		int2.Name = value.([]any)[1].(map[string]any)["interface"].(string)
		if as.IGP == "OSPF" {
			if len(value.([]any)[0].(map[string]any)) == 3 {
				int1.OSPFCost = int(value.([]any)[0].(map[string]any)["ospf_cost"].(float64))
			} else {
				int1.OSPFCost = -1
			}

			if len(value.([]any)[1].(map[string]any)) == 3 { 
				int2.OSPFCost = int(value.([]any)[1].(map[string]any)["ospf_cost"].(float64))
			} else {
				int2.OSPFCost = -1
			}
		}
		var link Link
		link[0], link[1] = int1, int2

		// On récupère l'indice du routeur dans la liste de routeurs
		var index1, index2 int
		for i, id := range as.RoutersId {
			if id == int1.RouterId {
				index1 = i
			}
			if id == int2.RouterId {
				index2 = i
			}
		}

		// On remplit la matrice d'adjacence de liens
		as.Adj[index1][index2] = &link
		as.Adj[index2][index1] = &link
	}

	as.LocalPrefs = make([][]int, len(as.RoutersId))
	for i := range as.RoutersId {
		as.LocalPrefs[i] = make([]int, len(as.RoutersId))
	}
 
	for _, value := range ASMap["local_prefs"].([]any) {
		id1 := int(value.(map[string]any)["id1"].(float64))
		id2 := int(value.(map[string]any)["id2"].(float64))
		pref := int(value.(map[string]any)["pref"].(float64))
	
		index1, index2 := -1, -1
		for i, v := range as.RoutersId {
			if id1 == v {
				index1 = i
			} else if id2 == v {
				index2 = i
			}
		}
		if index1 == -1 || index2 == -1 {
			fmt.Println("Error while loading local prefs")
			break
		}

		as.LocalPrefs[index1][index2] = pref
	}


	return as
}

func ImportAdmin(url string) []struct{ID int; IP string} {
	// Importing .json files
	file, _ := os.Open(url)
	defer file.Close()

	// Lecture du .json
	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// On caste le contenu en map de string
	var interfacesMap map[string]interface{}
	err = json.Unmarshal([]byte(data), &interfacesMap)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// On boucle dans la map pour extraire les strings
	var adminInterfaces []struct{ID int; IP string}
	for _, value := range interfacesMap["adminIP"].([]any) {
		routerId := int(value.(map[string]any)["id"].(float64))
		strIP := value.(map[string]any)["ip"].(string)
		adminInterfaces = append(adminInterfaces, struct{ID int; IP string}{routerId, strIP})
	}

	return adminInterfaces
}

