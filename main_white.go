package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
)

type ACNode struct {
	children map[rune]*ACNode
	fail     *ACNode
	isEnd    bool
	length   int
}

type AhoCorasick struct {
	root *ACNode
}

func NewAhoCorasick() *AhoCorasick {
	return &AhoCorasick{
		root: &ACNode{children: make(map[rune]*ACNode)},
	}
}

func (ac *AhoCorasick) Insert(word string) {
	node := ac.root
	for _, ch := range word {
		if _, ok := node.children[ch]; !ok {
			node.children[ch] = &ACNode{children: make(map[rune]*ACNode)}
		}
		node = node.children[ch]
	}
	node.isEnd = true
	node.length = len([]rune(word))
}

func (ac *AhoCorasick) BuildFailPointer() {
	queue := []*ACNode{ac.root}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for ch, child := range current.children {
			if current == ac.root {
				child.fail = ac.root
			} else {
				fail := current.fail
				for fail != nil {
					if next, ok := fail.children[ch]; ok {
						child.fail = next
						break
					}
					fail = fail.fail
				}
				if fail == nil {
					child.fail = ac.root
				}
			}
			queue = append(queue, child)
		}
	}
}

func (ac *AhoCorasick) FilterWithWhitelist(text string, whiteListedPositions []Position) string {
	node := ac.root
	runes := []rune(text)
	changes := false // 标记是否有替换发生

	log.Printf("开始过滤文本：%s", text)

	for i, ch := range runes {
		for node != ac.root && node.children[ch] == nil {
			node = node.fail
		}

		if next, ok := node.children[ch]; ok {
			node = next
		}

		tmp := node
		for tmp != ac.root {
			if tmp.isEnd {
				log.Printf("找到敏感词结束点，位于索引：%d，敏感词长度：%d", i, tmp.length)

				// 检查该位置是否在白名单中
				isInWhiteList := false
				for _, pos := range whiteListedPositions {
					if i >= pos.Start && i <= pos.End {
						isInWhiteList = true
						break
					}
				}

				if !isInWhiteList {
					for j := 0; j < tmp.length; j++ {
						if (i - j) >= 0 {
							log.Printf("替换字符：%c 为 '~'，位于索引：%d", runes[i-j], i-j)
							runes[i-j] = '~'
							changes = true
						}
					}
				}
			}
			tmp = tmp.fail
		}
	}

	if changes {
		log.Printf("过滤后的文本：%s", string(runes))
		return string(runes)
	}

	log.Printf("文本未发生改变：%s", text)
	return text
}

type Position struct {
	Start int
	End   int
}

func (ac *AhoCorasick) MatchPositions(text string) []Position {
	node := ac.root
	runes := []rune(text)
	positions := []Position{} // 用于储存匹配到的白名单词的位置

	log.Printf("开始匹配文本：%s", text)

	for i, ch := range runes {
		for node != ac.root && node.children[ch] == nil {
			node = node.fail
		}

		if next, ok := node.children[ch]; ok {
			node = next
		}

		tmp := node
		for tmp != ac.root {
			if tmp.isEnd {
				log.Printf("找到匹配词结束点，位于索引：%d，匹配词长度：%d", i, tmp.length)

				startPos := i - tmp.length + 1
				endPos := i
				positions = append(positions, Position{Start: startPos, End: endPos})

			}
			tmp = tmp.fail
		}
	}

	log.Printf("匹配到的位置：%v", positions)
	return positions
}

func loadWordsIntoAC(ac *AhoCorasick, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open the sensitive words file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ac.Insert(scanner.Text())
	}

	// 构建失败指针
	ac.BuildFailPointer()

	return scanner.Err()
}

func shenheHandler(ac *AhoCorasick, wac *AhoCorasick) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		word := r.URL.Query().Get("word")
		if word == "" {
			http.Error(w, "缺少 'word' 参数", http.StatusBadRequest)
			log.Printf("错误请求：缺少 'word' 参数")
			return
		}

		if len([]rune(word)) > 3000 {
			http.Error(w, "字符数超过最大限制（3000字符）", http.StatusBadRequest)
			log.Printf("错误请求：字符数超过最大限制（3000字符）")
			return
		}

		log.Printf("收到请求：word = %s", word)

		whiteListedPositions := wac.MatchPositions(word)
		log.Printf("白名单匹配位置：%v", whiteListedPositions)

		result := ac.FilterWithWhitelist(word, whiteListedPositions)
		log.Printf("过滤后的文本：%s", result)

		fmt.Fprintf(w, "过滤后的文本：%s", result)
	}
}

func main() {
	ac := NewAhoCorasick()
	wac := NewAhoCorasick()

	if err := loadWordsIntoAC(ac, "sensitive_words.txt"); err != nil {
		log.Fatalf("初始化敏感词库失败：%v", err)
		return
	}

	if err := loadWordsIntoAC(wac, "white.txt"); err != nil {
		log.Fatalf("初始化白名单词库失败：%v", err)
		return
	}

	http.HandleFunc("/shenhe", shenheHandler(ac, wac))

	log.Println("正在监听18000端口...")
	if err := http.ListenAndServe(":18000", nil); err != nil {
		log.Fatalf("启动服务器失败：%v", err)
	}
}
