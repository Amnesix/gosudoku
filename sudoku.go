package main

import (
	"fmt"
	gom "github.com/Amnesix/gomesure"
	gc "github.com/gbin/goncurses"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// URL d'un fichier de 2365 sudoku réputés très difficiles
const url string = "http://magictour.free.fr/top2365"

type Coordonnées struct {
	X int
	Y int
}
type Lifo struct {
	stack []Coordonnées
	pos int
}

var lifo Lifo

func (l *Lifo) Push(c Coordonnées) {
	(*l).stack = append((*l).stack, c)
	(*l).pos ++
}

func (l *Lifo) Pop() Coordonnées {
	(*l).pos --
	return (*l).stack[(*l).pos]
}

func (l *Lifo) Clear() {
	(*l).pos = 0
}

func (l *Lifo) isEmpty() bool {
	return (*l).pos == 0
}

var jeux []string
var courant int
var nbSudoku int
var mesure float64

// Pas de const []string en go -> fonction
func touches() []string {
	return []string{"zqsd", "hjkl", "Flèches"}
}

const (
	NORMAL = iota + 1
	BASE
	ERREUR
	NORMAL_
	BASE_
	ERREUR_
	VERT
	RVERT
	ROUGE
)
const (
	COL_NORMAL = iota
	COL_BASE
	COL_ERREUR
)
const CASE_VIDE = '.'

type Sudoku [9][9]byte
type SudokuB [9][9]bool

func (s *Sudoku) isOk(r, c int, v byte) bool {
	for i := 0; i < 9; i++ {
		if s[r][i] == v {
			return false
		}
	}
	for j := 0; j < 9; j++ {
		if s[j][c] == v {
			return false
		}
	}
	for j := 3 * (r / 3); j < 3*(1+r/3); j++ {
		for i := 3 * (c / 3); i < 3*(1+c/3); i++ {
			if s[j][i] == v {
				return false
			}
		}
	}
	return true
}

func (s *Sudoku) chercheVide() (int, int, bool) {
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if s[r][c] == CASE_VIDE {
				return r, c, true
			}
		}
	}
	return 0, 0, false
}

func (s *Sudoku) Solve() bool {
	r, c, ok := s.chercheVide()
	if !ok {
		return true
	}
	var v byte
	for v = '1'; v <= '9'; v++ {
		if s.isOk(r, c, v) {
			s[r][c] = v
			if s.Solve() {
				return true
			}
			s[r][c] = '.'
		}
	}
	return false
}

func couleurs() [2][3]int16 {
	return [2][3]int16{{NORMAL, BASE, ERREUR}, {NORMAL_, BASE_, ERREUR_}}
}

var timing gom.Mesure

type Screen struct {
	jeuTouches int
	screen     *gc.Window

	lines, columns int
	xCur, yCur     int
	offX, offY     int
}

type Jeu struct {
	scr       *Screen
	table     Sudoku
	resolve   Sudoku
	tbErreurs SudokuB
	possibles [9]SudokuB

	verify      bool
	possible    bool
	affPossible bool
	jeuTouches  int
	message     string
}

func (jeu *Jeu) nbPossibles(l, c int) int {
	nb := 0
	for i := 0; i < 9; i++ {
		if jeu.possibles[l][c][i] {
			nb++
		}
	}
	return nb
}

func (jeu *Jeu) verifier() int {
	err := 0
	for l := 0; l < 9; l++ {
		for c := 0; c < 9; c++ {
			jeu.tbErreurs[l][c] = false
		}
	}
	for l := 0; l < 9; l++ {
		for c := 0; c < 9; c++ {
			if jeu.tbErreurs[l][c] || jeu.resolve[l][c] == CASE_VIDE {
				continue
			}
			for i := 0; i < 9; i++ {
				v := jeu.resolve[i][c]
				if i != l && v != CASE_VIDE && v == jeu.resolve[l][c] {
					jeu.tbErreurs[l][c], jeu.tbErreurs[i][c] = true, true
				}
				v = jeu.resolve[l][i]
				if i != c && v != CASE_VIDE && v == jeu.resolve[l][c] {
					jeu.tbErreurs[l][c], jeu.tbErreurs[l][i] = true, true
				}
				v = jeu.resolve[(l/3)*3+i/3][(c/3)*3+i%3]
				if (l/3)*3+i/3 != l && (c/3)*3+i%3 != c && v != CASE_VIDE && v == jeu.resolve[l][c] {
					jeu.tbErreurs[l][c], jeu.tbErreurs[(l/3)*3+i/3][(c/3)*3+i%3] = true, true
				}
			}
		}
	}
	for l := 0; l < 9; l++ {
		for c := 0; c < 9; c++ {
			if jeu.tbErreurs[l][c] {
				err++
			}
		}
	}
	return err
}

func (jeu *Jeu) complet() bool {
	for l := 0; l < 9; l++ {
		for c := 0; c < 9; c++ {
			if jeu.resolve[l][c] == CASE_VIDE {
				return false
			}
		}
	}
	return jeu.verifier() == 0
}

func (jeu *Jeu) affiche() {
	s1 := "+-------+-------+-------+"
	fini := false
	menu := jeu.scr.offX + len(s1) + 8
	jeu.scr.screen.Clear()
	jeu.scr.screen.MovePrint(0, 7, "SUDOKU by Jean MORLET")
	jeu.scr.screen.MovePrint(1, 6, "=======================")

	if jeu.complet() {
		jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
		jeu.scr.screen.MovePrint(2, 6, "       ! BRAVO !")
		jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
		fini = true
	} else if mesure > 0 {
		jeu.scr.screen.AttrOn(gc.ColorPair(ERREUR))
		jeu.scr.screen.MovePrint(2, 6, "     ! IMPOSSIBLE !")
		jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
	} else {
		jeu.scr.screen.MovePrintf(2, 7, "Sudoku #%d", courant)
	}

	l := jeu.scr.offY
	for j := 0; j < 9; j++ {
		if j%3 == 0 {
			if fini {
				jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
			}
			jeu.scr.screen.MovePrint(l, jeu.scr.offX, s1)
			l++
			jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
		}
		c := jeu.scr.offX
		for i := 0; i < 9; i++ {
			if i%3 == 0 {
				if fini {
					jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
				}
				jeu.scr.screen.MoveAddChar(l, c, '|')
				c++
				jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
				jeu.scr.screen.MoveAddChar(l, c, ' ')
				c++
			}
			if jeu.affPossible {
				nb := jeu.nbPossibles(j, i)
				if jeu.resolve[j][i] == CASE_VIDE {
					if nb == 1 {
						jeu.scr.screen.AttrSet(gc.ColorPair(VERT))
					}
					jeu.scr.screen.MoveAddChar(l, c, gc.Char(nb+'0'))
					c++
				} else {
					jeu.scr.screen.MoveAddChar(l, c, ' ')
					c++
				}
			} else {
				v1 := jeu.table[j][i]
				v2 := jeu.resolve[j][i]
				switch {
				case jeu.tbErreurs[j][i]:
					jeu.scr.screen.AttrSet(gc.ColorPair(couleurs()[0][ERREUR-1]))
				case v1 != CASE_VIDE:
					jeu.scr.screen.AttrSet(gc.ColorPair(couleurs()[0][BASE-1]))
				default:
					jeu.scr.screen.AttrSet(gc.ColorPair(couleurs()[0][NORMAL-1]))
				}
				if jeu.resolve[jeu.scr.yCur][jeu.scr.xCur] != CASE_VIDE &&
					jeu.resolve[j][i] == jeu.resolve[jeu.scr.yCur][jeu.scr.xCur] {
					jeu.scr.screen.AttrOn(gc.A_BOLD)
				}
				if v2 == CASE_VIDE {
					jeu.scr.screen.MoveAddChar(l, c, gc.Char(v1))
				} else {
					jeu.scr.screen.MoveAddChar(l, c, gc.Char(v2))
				}
				c++
			}
			jeu.scr.screen.AttrSet(gc.ColorPair(NORMAL))
			jeu.scr.screen.MoveAddChar(l, c, ' ')
			c++
		}
		if fini {
			jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
		}
		jeu.scr.screen.AddChar('|')
		jeu.scr.screen.AttrOn(gc.ColorPair(NORMAL))
		l++
	}
	if fini {
		jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
	}
	jeu.scr.screen.MovePrint(l, jeu.scr.offX, s1)
	l++
	jeu.scr.screen.AttrOn(gc.ColorPair(NORMAL))
	if mesure > 0. {
		jeu.scr.screen.MovePrintf(l, jeu.scr.offX, "Found in = %.3fms", mesure)
	}

	l = 3
	jeu.scr.screen.MovePrint(l, menu, "1-9   Set figure at current position")
	l++
	jeu.scr.screen.MovePrint(l, menu, "c     Clear figure at current position")
	l++
	jeu.scr.screen.MovePrint(l, menu, "u     Undo")
	l++
	jeu.scr.screen.MovePrint(l, menu, "v     Real time check: ")
	l++
	if jeu.verify {
		jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
		jeu.scr.screen.Print("ON")
	} else {
		jeu.scr.screen.AttrOn(gc.ColorPair(ERREUR))
		jeu.scr.screen.Print("ON")
	}
	jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
	jeu.scr.screen.MovePrint(l, menu, "p     Show possible figure: ")
	l++
	if jeu.possible {
		jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
		jeu.scr.screen.Print("ON")
	} else {
		jeu.scr.screen.AttrOn(gc.ColorPair(ERREUR))
		jeu.scr.screen.Print("ON")
	}
	jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
	jeu.scr.screen.MovePrint(l, menu, "' '   Show nb possible: ")
	l++
	if jeu.affPossible {
		jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
		jeu.scr.screen.Print("ON")
	} else {
		jeu.scr.screen.AttrOn(gc.ColorPair(ERREUR))
		jeu.scr.screen.Print("ON")
	}
	jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
	jeu.scr.screen.AttrOn(gc.ColorPair(BASE))
	//jeu.scr.screen.Print(touches()[jeu.jeuTouches])
	jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
	jeu.scr.screen.MovePrint(l, menu, "*     Find solution")
	l++
	jeu.scr.screen.MovePrint(l, menu, "r     RaZ Sudoku")
	l++
	jeu.scr.screen.MovePrint(l, menu, "+     Next sudoku")
	l++
	jeu.scr.screen.MovePrint(l, menu, "-     Previous sudoku")
	l++
	jeu.scr.screen.MovePrint(l, menu, "x     Exit")
	l++

	if jeu.possible {
		for i := 0; i < 9; i++ {
			if jeu.possibles[jeu.scr.yCur][jeu.scr.xCur][i] {
				jeu.scr.screen.AttrOn(gc.ColorPair(VERT))
			}
			jeu.scr.screen.MovePrintf(i+5, jeu.scr.offX+28, "%c", byte('1'+i))
			jeu.scr.screen.AttrOff(gc.ColorPair(NORMAL))
		}
	}
	//jeu.scr.screen.MovePrint(16, 7, jeu.message)
	jeu.scr.screen.Move(jeu.scr.offY+4*jeu.scr.yCur/3+1, jeu.scr.offX+2*(4*jeu.scr.xCur/3+1))
	jeu.scr.screen.Refresh()
}

func (jeu *Jeu) setpossibles() {
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			if jeu.resolve[y][x] != CASE_VIDE {
				continue
			}
			for i := 0; i < 9; i++ {
				jeu.possibles[y][x][i] = true
			}
			for i := 0; i < 9; i++ {
				for l := 0; l < 9; l++ {
					if jeu.resolve[l][x] == byte('1'+i) {
						jeu.possibles[y][x][i] = false
						break
					}
				}
				for c := 0; c < 9; c++ {
					if jeu.resolve[y][c] == byte('1'+i) {
						jeu.possibles[y][x][i] = false
						break
					}
				}
				for l := 0; l < 3; l++ {
					for c := 0; c < 3; c++ {
						if jeu.resolve[(y/3)*3+l][(x/3)*3+c] == byte('1'+i) {
							jeu.possibles[y][x][i] = false
						}
					}
				}
			}
		}
	}
}

func (jeu *Jeu) majPossible(l, c, v int) {
	v--
	for i := 0; i < 9; i++ {
		jeu.possibles[i][c][v] = false
		jeu.possibles[l][i][v] = false
		jeu.possibles[(l/3)*3+i/3][(c/3)*3+i%3][v] = false
	}
}

func (jeu *Jeu) reset() {
	for l := 0; l < 9; l++ {
		for c := 0; c < 9; c++ {
			jeu.resolve[l][c] = jeu.table[l][c]
			jeu.tbErreurs[l][c] = false
		}
	}
}

//chiffre : Saisie d'un chiffre.
func (jeu *Jeu) chiffre(key string) {
	if jeu.table[jeu.scr.yCur][jeu.scr.xCur] != CASE_VIDE {
		return
	}
	c := key[0]
	jeu.resolve[jeu.scr.yCur][jeu.scr.xCur] = c
	val, _ := strconv.ParseInt(key, 10, 0)
	jeu.majPossible(jeu.scr.yCur, jeu.scr.xCur, int(val))
	for i := 0; i < 9; i++ {
		jeu.possibles[jeu.scr.yCur][jeu.scr.xCur][i] = false
	}
	lifo.Push(Coordonnées{jeu.scr.xCur, jeu.scr.yCur})
}

func (jeu *Jeu) clear() {
	jeu.resolve[jeu.scr.yCur][jeu.scr.xCur] = jeu.table[jeu.scr.yCur][jeu.scr.xCur]
}

func (jeu *Jeu) undo() {
	if lifo.isEmpty() { return }
	coordonnées := lifo.Pop()
	jeu.scr.xCur = coordonnées.X
	jeu.scr.yCur = coordonnées.Y
	jeu.clear()
}

func (jeu *Jeu) new(sudoku string) {
	for r := 0; r < 9; r++ {
		line := sudoku[r*9 : (r+1)*9]
		for c := 0; c < 9; c++ {
			jeu.table[r][c] = line[c]
		}
	}
}

func (jeu *Jeu) init() {
	var err error
	jeu.scr.screen, err = gc.Init()
	if err != nil {
		log.Fatal("init:", err)
	}
	if !gc.HasColors() {
		log.Fatal("Sudoky requires a colour capable terminal")
	}
	if err := gc.StartColor(); err != nil {
		log.Fatal(err)
	}
	gc.UseEnvironment(true)
	gc.Echo(false)
	gc.Raw(true)
	jeu.scr.screen.Keypad(true)
	ip := func(n, fg, bg int16) {
		if err := gc.InitPair(n, fg, bg); err != nil {
			log.Fatal("InitPair failed: ", err)
		}
	}
	ip(NORMAL, gc.C_WHITE, gc.C_BLACK)
	ip(BASE, gc.C_CYAN, gc.C_BLACK)
	ip(ERREUR, gc.C_YELLOW, gc.C_RED)
	ip(NORMAL_, gc.C_BLACK, gc.C_YELLOW)
	ip(BASE_, gc.C_BLUE, gc.C_YELLOW)
	ip(ERREUR_, gc.C_RED, gc.C_YELLOW)
	ip(VERT, gc.C_GREEN, gc.C_BLACK)
	ip(RVERT, gc.C_BLACK, gc.C_GREEN)
	ip(ROUGE, gc.C_RED, gc.C_BLACK)

	jeu.scr.offX = 5
	jeu.scr.offY = 3
	jeu.scr.xCur = 0
	jeu.scr.yCur = 0
	jeu.affPossible = false
	jeu.possible = false
	jeu.verify = false
	jeu.jeuTouches = 1

	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	buffer, err := io.ReadAll(res.Body)
	res.Body.Close()
	for i := 0; i < 100; i++ {
		fmt.Printf("%c", buffer[i])
	}
	jeux = strings.Split(string(buffer), "\n")
	courant = 0
	nbSudoku = len(jeux) - 1
}

func main() {

	var jeu Jeu
	jeu.scr = new(Screen)
	(&jeu).init()
	defer gc.End()

	lines, _ := strconv.ParseInt(os.Getenv("LINES"), 10, 64)
	jeu.scr.lines = int(lines)
	columns, _ := strconv.ParseInt(os.Getenv("COLUMNS"), 10, 64)
	jeu.scr.columns = int(columns)

	jeu.new(jeux[courant])

	jeu.reset()
	jeu.affiche()

	for {
		mesure = 0
		c := jeu.scr.screen.GetChar()
		jeu.message = ""
		key := gc.KeyString(c)
		switch key {
		case "x":
			return
		case "down", "j", "s":
			jeu.scr.yCur = (jeu.scr.yCur + 1) % 9
		case "up", "k", "z":
			jeu.scr.yCur = (jeu.scr.yCur + 8) % 9
		case "right", "l", "d":
			jeu.scr.xCur = (jeu.scr.xCur + 1) % 9
		case "left", "h", "q":
			jeu.scr.xCur = (jeu.scr.xCur + 8) % 9
		case "*":
			timing.Starttime()
			jeu.resolve.Solve()
			timing.Elapsedtime()
			mesure = timing.GetSecondes()
		case "c":
			jeu.clear()
		case "v":
			jeu.verify = !jeu.verify
			for l := 0; l < 9; l++ {
				for c := 0; c < 9; c++ {
					jeu.tbErreurs[l][c] = false
				}
			}
		case "r":
			jeu.reset()
		case "p":
			jeu.possible = !jeu.possible
			if jeu.possible {
				jeu.setpossibles()
			}
		case " ":
			jeu.affPossible = !jeu.affPossible
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			jeu.chiffre(key)
		case "u":
			jeu.undo()
		case "+":
			courant = (courant + 1) % nbSudoku
			jeu.new(jeux[courant])
			jeu.reset()
		case "-":
			courant = (courant + nbSudoku - 1) % nbSudoku
			jeu.new(jeux[courant])
			jeu.reset()
		}
		if jeu.verify {
			jeu.verifier()
		}
		jeu.affiche()
	}
}
