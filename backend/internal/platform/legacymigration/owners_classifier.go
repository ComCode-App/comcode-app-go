package legacymigration

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var ownerFree = wordSet(
	"gmail.com hotmail.com outlook.com yahoo.com icloud.com yandex.com yandex.com.tr msn.com " +
		"live.com hotmail.com.tr outlook.com.tr me.com mynet.com windowslive.com protonmail.com " +
		"gmail.com.tr yahoo.com.tr mail.ru googlemail.com aol.com mac.com proton.me hotmail.de " +
		"gmx.de gmx.net web.de",
)
var ownerBlock = wordSet("comcode.app drive.google.com sites.google.com linktr.ee wa.me bit.ly")
var ownerSuffix = wordSet("com net org info biz co tr uk de io app software gen bel edu gov k12 av tv me")
var ownerStop = wordSet(
	"ltd sti şti as a.s a.ş anonim sirketi şirketi limited san tic ve ltd.şti inc llc gmbh co " +
		"dis ticaret sanayi insaat ins as. ltd. sti. a.s. a.ş. danismanlik hizmetleri grup group " +
		"holding gayrimenkul emlak office ofisi ofis sanat the bureau burosu hukuk",
)
var ownerTokens = regexp.MustCompile(`[^a-z0-9]+`)

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, v := range strings.Fields(s) {
		m[v] = true
	}
	return m
}

func normalizeOrg(s string) string {
	s = strings.ToLower(RepairText(s))
	s = strings.NewReplacer(
		"ç", "c", "ğ", "g", "ı", "i",
		"ö", "o", "ş", "s", "ü", "u",
		"â", "a", "î", "i", "û", "u",
	).Replace(s)
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		if r < unicode.MaxASCII {
			b.WriteRune(r)
		}
	}
	out := []string{}
	for _, t := range ownerTokens.Split(b.String(), -1) {
		if t != "" && !ownerStop[t] {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

func ownerBrand(d string) string {
	if d == "" || ownerFree[d] || ownerBlock[d] {
		return ""
	}
	for _, blocked := range []string{"instagram", "linkedin", "facebook", "twitter", "youtube", "sahibinden.com"} {
		if strings.Contains(d, blocked) {
			return ""
		}
	}
	labels := []string{}
	for _, l := range strings.Split(d, ".") {
		if l != "" {
			labels = append(labels, l)
		}
	}
	for len(labels) > 1 && ownerSuffix[labels[len(labels)-1]] {
		labels = labels[:len(labels)-1]
	}
	if len(labels) == 0 {
		return ""
	}
	return labels[len(labels)-1]
}

func emailDomain(s string) string {
	_, d, ok := strings.Cut(strings.ToLower(strings.TrimSpace(RepairText(s))), "@")
	if !ok {
		return ""
	}
	return d
}
func urlDomain(s string) string {
	s = strings.ToLower(strings.TrimSpace(RepairText(s)))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	return strings.Split(strings.TrimPrefix(s, "www."), "/")[0]
}

type ownerCandidate struct {
	ID, Email, Name, Surname, Org, Title, Group, Skip string
	ProfileID                                         int64
}

func ownerSource(row map[string]string) ownerCandidate {
	v := jsonObject(repairEncoding(row["vcard"]))
	name := strings.Fields(RepairText(row["name"]))
	firstName, surname := "", ""
	if len(name) > 0 {
		firstName = name[0]
		surname = strings.Join(name[1:], " ")
	}
	c := ownerCandidate{
		ID: row["user_id"], Email: strings.ToLower(strings.TrimSpace(RepairText(row["email"]))),
		Name: first(v["firstname"], firstName), Surname: first(v["lastname"], surname),
		Org: first(v["organization"], RepairText(row["companies"])), Title: v["title"],
	}
	keys := []string{
		ownerBrand(emailDomain(v["workemail"])),
		ownerBrand(emailDomain(c.Email)),
		normalizeOrg(c.Org),
		ownerBrand(urlDomain(v["workurl"])),
	}
	for i, k := range keys {
		if k == "" || (i == 2 && len(k) < 3) {
			continue
		}
		c.Group = "b:" + k
		if i == 2 {
			c.Group = "o:" + k
		}
		break
	}
	if c.Group == "" {
		c.Group = "solo:" + c.ID
	}
	if row["active"] == "0" || row["deactivated"] == "1" {
		c.Skip = "inactive"
	}
	return c
}

// classifyOwners follows classify_reference.py, including first-seen tie breaking.
func classifyOwners(users []ownerCandidate) map[string][]int {
	groups := map[string][]int{}
	order := []string{}
	add := func(k string, i int) {
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}
	for i, c := range users {
		if c.Skip == "" {
			add(c.Group, i)
		}
	}
	for _, k := range append([]string{}, order...) {
		if !strings.HasPrefix(k, "b:") {
			continue
		}
		ids := groups[k]
		named := []int{}
		common := map[string]bool{}
		tokens := map[int]map[string]bool{}
		for _, i := range ids {
			ts := wordSet(normalizeOrg(users[i].Org))
			tokens[i] = ts
			if len(ts) == 0 {
				continue
			}
			if len(named) == 0 {
				for t := range ts {
					common[t] = true
				}
			} else {
				for t := range common {
					if !ts[t] {
						delete(common, t)
					}
				}
			}
			named = append(named, i)
		}
		if len(named) < 4 {
			continue
		}
		sub := map[string][]int{}
		subOrder := []string{}
		for _, i := range named {
			rest := []string{}
			for t := range tokens[i] {
				if !common[t] {
					rest = append(rest, t)
				}
			}
			sort.Strings(rest)
			s := strings.Join(rest, " ")
			if _, ok := sub[s]; !ok {
				subOrder = append(subOrder, s)
			}
			sub[s] = append(sub[s], i)
		}
		big := []string{}
		largest := ""
		for _, s := range subOrder {
			if s != "" && len(sub[s]) >= 3 {
				big = append(big, s)
				if len(sub[s]) > len(sub[largest]) || largest == "" {
					largest = s
				}
			}
		}
		if len(big) < 2 {
			continue
		}
		delete(groups, k)
		assigned := map[int]bool{}
		for _, s := range big {
			for _, i := range sub[s] {
				add(k+"/"+s, i)
				assigned[i] = true
			}
		}
		for _, i := range ids {
			if !assigned[i] {
				add(k+"/"+largest, i)
			}
		}
	}
	dominant := map[string]string{}
	for _, k := range order {
		ids := groups[k]
		if !strings.HasPrefix(k, "b:") || len(ids) < 2 {
			continue
		}
		counts := map[string]int{}
		winner := ""
		for _, i := range ids {
			s := normalizeOrg(users[i].Org)
			if s != "" {
				counts[s]++
			}
		}
		for _, i := range ids {
			s := normalizeOrg(users[i].Org)
			if counts[s] > counts[winner] {
				winner = s
			}
		}
		if winner != "" && dominant[winner] == "" {
			dominant[winner] = k
		}
	}
	for _, k := range order {
		if strings.HasPrefix(k, "o:") {
			if target := dominant[k[2:]]; target != "" {
				groups[target] = append(groups[target], groups[k]...)
				delete(groups, k)
			}
		}
	}
	return groups
}

func legacyIDLess(a, b string) bool {
	ai, ae := strconv.ParseInt(a, 10, 64)
	bi, be := strconv.ParseInt(b, 10, 64)
	if ae == nil && be == nil {
		return ai < bi
	}
	return a < b
}

func companyOwner(ids []int, users []ownerCandidate) int {
	keywords := []string{
		"ceo",
		"founder",
		"kurucu",
		"genel mudur",
		"general manager",
		"owner",
		"sahibi",
		"yonetici ortak",
		"managing director",
		"broker/owner",
	}
	for _, i := range ids {
		title := normalizeOrg(users[i].Title)
		for _, k := range keywords {
			if strings.Contains(title, k) {
				return i
			}
		}
	}
	return ids[0]
}
