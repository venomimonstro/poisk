package query

import "strings"

var enToRU = map[rune]rune{
	'q':'й','w':'ц','e':'у','r':'к','t':'е','y':'н','u':'г','i':'ш','o':'щ','p':'з','[':'х',']':'ъ',
	'a':'ф','s':'ы','d':'в','f':'а','g':'п','h':'р','j':'о','k':'л','l':'д',';':'ж','\'':'э',
	'z':'я','x':'ч','c':'с','v':'м','b':'и','n':'т','m':'ь',',':'б','.':'ю',
}

var ruToEN map[rune]rune

func init() {
	ruToEN = make(map[rune]rune, len(enToRU))
	for en, ru := range enToRU { ruToEN[ru] = en }
}

func swapLayout(s string) string {
	var b strings.Builder
	for _, r := range s {
		if mapped, ok := enToRU[r]; ok { b.WriteRune(mapped); continue }
		if mapped, ok := ruToEN[r]; ok { b.WriteRune(mapped); continue }
		b.WriteRune(r)
	}
	return b.String()
}

func looksWrongLayout(s string) bool {
	latin, cyr := 0, 0
	for _, r := range s {
		if _, ok := enToRU[r]; ok { latin++ }
		if _, ok := ruToEN[r]; ok { cyr++ }
	}
	return latin >= 3 || cyr >= 3
}
