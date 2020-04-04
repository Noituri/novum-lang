// GoLang Scanner inspired
package main

import (
	"errors"
	"strconv"
	"unicode"
	"unicode/utf8"
)

type Token int

const (
	TokEOF Token = iota // End of string/file
	TokIdentifier 		// Identifier
	TokNumber     		// number
	TokStr        		// string
	TokLParen     		// (
	TokRParen     		// )
	TokLBrace			// {
	TokRBrace			// }
	TokEqual      		// ==
	TokAssign     		// =
	TokAttribute  		// #[attr1 = 0]
	TokAtom       		// :atom

	KWBegin
	TokIn         		// loop - element in array
	TokForLoop          // for/while/foreach loop
	TokElse       		// If else
	TokIf         		// If
	TokFalse			// boolean false
	TokTrue				// boolean true
	TokExtern     		// extern procedure
	TokUnknown    		// Not specified type
	TokReturn     		// procedure return
	TokFunction   		// function
	KWEnd
)

var tokens = map[Token]string {
	TokAtom: "ATOM",
	TokUnknown: "UNKNOWN",
	TokEOF: "EOF",
	TokIdentifier: "IDENT",
	TokNumber: "NUMBER",
	TokStr: "STRING",
	TokAttribute: "ATTRIBUTE",
	TokExtern: "@",
	TokFunction: "fun",
	TokReturn: "return",
	TokTrue: "true",
	TokFalse: "false",
	TokIf: "if",
	TokElse: "else",
	TokForLoop: "for",
	TokIn: "in",
	TokLParen: "(",
	TokRParen: ")",
	TokLBrace: "{",
	TokRBrace: "}",
	TokEqual: "==",
	TokAssign: "=",
}

var keywords map[string]Token

func init() {
	keywords = make(map[string]Token)
	for i := KWBegin + 1; i < KWEnd; i++ {
		keywords[tokens[i]] = i
	}
}

func Lookup(identifier string) Token {
	if tok, found := keywords[identifier]; found {
		return tok
	}
	return TokIdentifier
}

type Lexer struct {
	source        string
	token	 	  Token
	identifier    string
	numVal        float64
	strVal        string
	offsetChar	  int
	forwardOffset int
	line		  int
	col			  int
	lastChar      rune
	isEOF         bool
	ignoreNewLine bool
	ignoreSpace   bool
	ignoreAtoms   bool
	isFloat       bool
}

func (l Lexer) New(source string) Lexer {
	lexer := Lexer{
		source: source,
		offsetChar: 0,
		forwardOffset: 0,
		line: 1,
	}
	_ = lexer.nextChar()
	if lexer.lastChar == 0xFEFF {
		_ = lexer.nextChar()
	}

	return lexer
}

func (l *Lexer) nextChar() error {
	if l.forwardOffset < len(l.source) {
		l.offsetChar = l.forwardOffset
		if l.lastChar == '\n' {
			l.col = 0
			l.line++
		}
		l.col++
		ch := rune(l.source[l.forwardOffset])
		addOffset := 1
		if ch == 0 {
			return errors.New("null character")
		} else if ch >= utf8.RuneSelf {
			ch, addOffset = utf8.DecodeRune([]byte(l.source[l.forwardOffset:]))
			if ch == utf8.RuneError && addOffset == 1 {
				return errors.New("illegal UTF-8 encoding")
			} else if ch == 0xFEFF && l.offsetChar > 0 {
				return errors.New("0xFEFF is only allowed as a first character")
			}
		}

		l.lastChar = ch
		l.forwardOffset += addOffset
		return nil
	}

	return errors.New("eof")
}

func (l *Lexer) peek() byte {
	if l.forwardOffset < len(l.source) {
		return l.source[l.forwardOffset]
	}
	return 0
}

func (l *Lexer) removeSpace() (stopLexing bool) {
	for ((l.lastChar == 32 || l.lastChar == '\t') && l.ignoreSpace) || ((l.lastChar == 10 || l.lastChar == 13) && l.ignoreNewLine) {
		if l.nextChar() != nil {
			l.token = TokEOF
			return true
		}
	}

	return false
}

func (l *Lexer) isAlphabetic() (stopLexing bool) {
	if unicode.IsLetter(l.lastChar) {
		l.identifier = string(l.lastChar)


		for unicode.IsLetter(l.lastChar) || l.lastChar == '_' {
			if l.nextChar() != nil {
				break
			}
			l.identifier += string(l.lastChar)

		}

		l.token = Lookup(l.identifier)
		return true
	}
	return false
}

func (l *Lexer) isDigit() (stopLexing bool) {
	if unicode.IsNumber(l.lastChar) || l.lastChar == '.' {
		tempStr := ""
		l.isFloat = l.lastChar == '.'

		for {
			tempStr += string(l.lastChar)
			if l.nextChar() != nil {
				l.token = TokEOF
				return true
			}

			if l.lastChar == '.' {
				if l.isFloat {
					panic("Invalid use of '.'")
				} else {
					l.isFloat = true
				}
			}

			if !unicode.IsNumber(l.lastChar) && l.lastChar != 46 {
				break
			}
		}

		l.numVal, _ = strconv.ParseFloat(tempStr, 64)
		l.token = TokNumber
		return true
	}

	return false
}

func (l *Lexer) isComment() (stopLexing bool) {
	if l.lastChar == '/' {
		l.isEOF = l.nextChar() != nil
		ch := l.peek()
		if ch == '/' {
			for {
				if l.nextChar() != nil {
					l.token = TokEOF
					return true
				}

				if l.lastChar == 13 || l.lastChar == 10 {
					break
				}
			}

			l.NextToken()
			return true
		} else if ch == '*' {
			for {
				if l.nextChar() != nil {
					l.token = TokEOF
					return true
				}

				if l.lastChar == '*' {
					if l.nextChar() != nil {
						l.token = TokEOF
						return true
					}

					if l.lastChar == '/' {
						if l.nextChar() != nil {
							l.token = TokEOF
							return true
						}
						break
					}
				}
			}

			l.NextToken()
			return true
		}
	}
	return false
}

func (l *Lexer) isParen() (stopLexing bool) {
	if l.lastChar == '(' {
		l.isEOF = l.nextChar() != nil
		l.token = TokLParen
		return true
	}

	if l.lastChar == ')' {
		l.isEOF = l.nextChar() != nil
		l.token = TokRParen
		return true
	}

	return false
}

func (l *Lexer) isExtern() (stopLexing bool) {
	if l.lastChar == '@' {
		l.isEOF = l.nextChar() != nil
		l.token = TokExtern
		return true
	}

	return false
}

func (l *Lexer) isStr() (stopLexing bool) {
	if l.lastChar == '"' {
		if l.nextChar() != nil {
			l.token = TokEOF
			return true
		}

		l.strVal = ""
		for l.lastChar != '"' {
			l.strVal += string(l.lastChar)
			if l.nextChar() != nil {
				l.isEOF = true
				break
			}

		}

		l.isEOF = l.nextChar() != nil

		l.token = TokStr
		return true
	}

	return false
}

func (l *Lexer) isEqual() (stopLexing bool) {
	if l.lastChar == '=' {
		if l.nextChar() != nil {
			l.token = TokEOF
			return true
		}

		if l.lastChar != '=' {
			l.token = TokAssign
			return true
		}

		l.isEOF = l.nextChar() != nil
		l.token = TokEqual
		return true
	}

	return false
}

//func (l *Lexer) isAttribute() (stopLexing bool) {
//	if l.lastChar == '#' {
//		if l.nextChar() != nil {
//			l.CurrentToken.kind = TokEOF
//			l.CurrentToken.val = -1
//			return true
//		}
//
//		if l.LastChar == '[' {
//			l.isEOF = l.nextChar() != nil
//			l.CurrentToken.kind = TokAttribute
//			l.CurrentToken.val = -1
//			return true
//		}
//	}
//
//	return false
//}

//func (l *Lexer) isAtom() (stopLexing bool) {
//	if l.LastChar == ':' {
//		oldToken := l.CurrentToken
//		oldChar := l.CurrentChar
//
//		l.ignoreSpace = false
//		l.ignoreNewLine = false
//
//		if l.nextChar() != nil {
//			l.CurrentToken.kind = TokEOF
//			l.CurrentToken.val = -1
//			l.ignoreSpace = true
//			l.ignoreNewLine = true
//			return true
//		}
//
//		atom := ""
//		for unicode.IsLetter(rune(l.LastChar)) || l.LastChar == '_' {
//			atom += string(rune(l.LastChar))
//
//			if l.nextChar() != nil {
//				break
//			}
//		}
//
//		l.ignoreSpace = true
//		l.ignoreNewLine = true
//
//		if atom == "" {
//			l.LastChar = ':'
//			l.CurrentToken = oldToken
//			l.CurrentChar = oldChar
//			return false
//		}
//
//		l.CurrentToken.kind = TokAtom
//		l.CurrentToken.val = -1
//		l.Identifier = ":" + atom
//		return true
//	}
//
//	return false
//}

func (l *Lexer) NextToken() {
	if l.isEOF {
		l.token = TokEOF
		return
	}

	if l.removeSpace() {
		return
	}

	//if l.isAttribute() {
	//	return
	//}

	if l.isAlphabetic() {
		return
	}

	if l.isExtern() {
		return
	}

	if l.isStr() {
		return
	}

	if l.isDigit() {
		return
	}

	if l.isEqual() {
		return
	}

	//if !l.ignoreAtoms {
	//	if l.isAtom() {
	//		return
	//	}
	//}

	if l.isComment() {
		return
	}

	if l.isParen() {
		return
	}

	l.isEOF = l.nextChar() != nil

	l.token = TokUnknown
}
