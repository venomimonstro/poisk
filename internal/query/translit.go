package query

import "strings"

var ruToLatin = map[rune]string{
	'а':"a",'б':"b",'в':"v",'г':"g",'д':"d",'е':"e",'ё':"yo",'ж':"zh",'з':"z",'и':"i",'й':"y",
	'к':"k",'л':"l",'м':"m",'н':"n",'о':"o",'п':"p",'р':"r",'с':"s",'т':"t",'у':"u",'ф':"f",
	'х':"kh",'ц':"ts",'ч':"ch",'ш':"sh",'щ':"sch",'ъ':"",'ы':"y",'ь':"",'э':"e",'ю':"yu",'я':"ya",
}

func transliterateRU(s string) string {
	var b strings.Builder
	for _, r := range s {
		if v, ok := ruToLatin[r]; ok { b.WriteString(v) } else { b.WriteRune(r) }
	}
	return b.String()
}

func transliterateLatinToRU(s string) string {
	// Deliberately deterministic and conservative: longest tokens first.
	replacer := strings.NewReplacer(
		"sch","щ","sh","ш","ch","ч","zh","ж","kh","х","ts","ц","yo","ё","yu","ю","ya","я",
		"a","а","b","б","v","в","g","г","d","д","e","е","z","з","i","и","y","й","k","к",
		"l","л","m","м","n","н","o","о","p","п","r","р","s","с","t","т","u","у","f","ф",
	)
	return replacer.Replace(s)
}
