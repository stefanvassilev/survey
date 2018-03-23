package terminal

import (
	"os"
	"unicode"
)

type RuneReader struct {
	Input *os.File

	state runeReaderState
}

func NewRuneReader(input *os.File) *RuneReader {
	return &RuneReader{
		Input: input,
		state: newRuneReaderState(input),
	}
}

func (rr *RuneReader) ReadLine(mask rune) ([]rune, error) {
	line := []rune{}
	// we only care about horizontal displacements from the origin so start counting at 0
	index := 0

	// we get the terminal width and height (if resized after this point the property might become invalid)
	terminalSize, _ := Size()
	for {
		// wait for some input
		r, _, err := rr.ReadRune()
		if err != nil {
			return line, err
		}
		// we set the current location of the cursor and update it by every key press
		cursorCurrent, err := CursorLocation()

		// if the user pressed enter or some other newline/termination like ctrl+d
		if r == '\r' || r == '\n' || r == KeyEndTransmission {
			// go to the beginning of the next line
			Print("\r\n")

			// we're done processing the input
			return line, nil
		}

		// if the user interrupts (ie with ctrl+c)
		if r == KeyInterrupt {
			// go to the beginning of the next line
			Print("\r\n")

			// we're done processing the input, and treat interrupt like an error
			return line, InterruptErr
		}

		// allow for backspace/delete editing of inputs
		if r == KeyBackspace || r == KeyDelete {
			// and we're not at the beginning of the line
			if index > 0 && len(line) > 0 {
				// if we are at the end of the word
				if index == len(line) {
					// just remove the last letter from the internal representation
					line = line[:len(line)-1]
					// go back one
					if cursorCurrent.X == 1 {
						CursorPreviousLine(1)
						CursorForward(int(terminalSize.X))
					} else {
						CursorBack(1)
					}

					// clear the rest of the line
					EraseLine(ERASE_LINE_END)
				} else {
					// we need to remove a character from the middle of the word

					// remove the current index from the list
					line = append(line[:index-1], line[index:]...)

					// save cur cursor location
					CursorSave()

					// clear the rest of the line
					CursorBack(1)

					// print what comes after
					for _, char := range line[index - 1:] {
						//Erase symbols which are left over from older print
						EraseLine(ERASE_LINE_END)
						// print characters to the new line appropriately
						Printf("%c", char)

					}
					// erase what's left from last print
					Printf("\x1bE")
					EraseLine(ERASE_LINE_END)

					// restore cursor
					CursorRestore()
					if cursorCurrent.CursorIsAtLineBegin(){
						CursorPreviousLine(1)
						CursorForward(int(terminalSize.X))
					} else {
						CursorBack(1)
					}
				}

				// decrement the index
				index--
			} else {
				// otherwise the user pressed backspace while at the beginning of the line
				soundBell()
			}

			// we're done processing this key
			continue
		}

		// if the left arrow is pressed
		if r == KeyArrowLeft {
			// if we have space to the left
			if index > 0 {
				//move the cursor to the prev line if necessary
				if cursorCurrent.CursorIsAtLineBegin() {
					CursorUp(1)
					CursorForward(int(terminalSize.X))
				} else {
					CursorBack(1)
				}
				 //decrement the index
				index--

			} else {
				// otherwise we are at the beginning of where we started reading lines
				// sound the bell
				soundBell()
			}

			// we're done processing this key press
			continue
		}

		// if the right arrow is pressed
		if r == KeyArrowRight {
			// if we have space to the right
			if index < len(line) {
				// move the cursor to the next line if necessary
				if cursorCurrent.CursorIsAtLineEnd(terminalSize){
					CursorNextLine(1)
				} else {
					CursorForward(1)
				}
				index++

			} else {
				// otherwise we are at the end of the word and can't go past
				// sound the bell
				soundBell()
			}

			// we're done processing this key press
			continue
		}
		// the user pressed one of the special keys
		if r == SpecialKeyHome {
			for index > 0 {
				if cursorCurrent.CursorIsAtLineBegin() {
					CursorPreviousLine(1)
					CursorForward(int(terminalSize.X))
					cursorCurrent.X = terminalSize.X
					cursorCurrent.Y--

				} else {
					CursorBack(1)
					cursorCurrent.X--
				}
				index--
			}
			continue
		} else if r == SpecialKeyEnd {
			for index != len(line) {
				if cursorCurrent.CursorIsAtLineEnd(terminalSize){
					CursorNextLine(1)
					cursorCurrent.X = 1
					cursorCurrent.Y++

				} else {
					CursorForward(1)
					cursorCurrent.X++
				}
				index++
			}
			continue
		} else if r == SpecialKeyDelete {
			// if index at the end of the line nothing to delete
			if index != len(line){
				CursorSave()
				// remove the symbol after the cursor
				line = append(line[:index], line[index + 1:]...)
				// print the updated line
				for _, char := range line[index:] {
					EraseLine(ERASE_LINE_END)
					Printf("%c", char)
				}
				// erase what's left on last line
				Printf("\x1bE")
				EraseLine(ERASE_LINE_END)

				// restore cursor
				CursorRestore()
				if len(line) == 0 || index == len(line){
					EraseLine(ERASE_LINE_END)
				}
			}
			continue
		}

		// if the letter is another escape sequence
		if unicode.IsControl(r) || r == IgnoreKey {
			// ignore it
			continue
		}

		// the user pressed a regular key

		// if we are at the end of the line
		if index == len(line) {
			// just append the character at the end of the line
			line = append(line, r)
			// save the location of the cursor
			index++
			// if we don't need to mask the input
			if mask == 0 {
				// just print the character the user pressed
				Printf("%c", r)
			} else {
				// otherwise print the mask we were given
				Printf("%c", mask)
			}
		} else {
			// we are in the middle of the word so we need to insert the character the user pressed
			line = append(line[:index], append([]rune{r}, line[index:]...)...)
			// save the current position of the cursor
			CursorSave()

			// visually insert the character by deleting the rest of the line
			EraseLine(ERASE_LINE_END)
			// print the rest of the word after
			for _, char := range line[index:] {
				// print characters to the new line appropriately
				if  cursorCurrent.X == terminalSize.X {
					CursorNextLine(1)
					EraseLine(ERASE_LINE_END)
					cursorCurrent.Y++
					cursorCurrent.X = 1
				}
				// if we don't need to mask the input
				if mask == 0 {
					// just print the character the user pressed
					Printf("%c", char)
				} else {
					// otherwise print the mask we were given
					Printf("%c", mask)
				}
				cursorCurrent.X++
			}
			// leave the cursor where the user left it
			CursorRestore()
			cursorCurrent, _ = CursorLocation()
			if cursorCurrent.X == terminalSize.X {
				CursorNextLine(1)
				//fmt.Print(len(line), "index", index)
			} else {
				CursorForward(1)
			}

			// increment the index
			index++
		}
	}
}
