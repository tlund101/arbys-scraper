package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func main() {
	recommendId := "4434084855901"
	recommendationsUrl := "https://arbysshop.com/recommendations/products?&section_id=product-recommendations&product_id="
	recommendMap := make(map[string]Product)
	callStack := []string{}
	callStack = append(callStack, recommendationsUrl+recommendId)
	processedRequests := make(map[string]bool)

	for len(callStack) > 0 {
		if processedRequests[callStack[len(callStack)-1]] {
			callStack = callStack[:len(callStack)-1]
			continue
		}

		// Pop from call stack
		resp, err := fetchRecommendations(callStack[len(callStack)-1])
		processedRequests[callStack[len(callStack)-1]] = true
		callStack = callStack[:len(callStack)-1]

		if err != nil {
			fmt.Println("Error fetching recommendations:", err)
			return
		}

		// Process the response here
		callStack = append(callStack, processResponse(resp, recommendMap)...)
	}

	// Write out to HTML file
	file, err := os.Create(filepath.Join("index.html"))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	for _, product := range recommendMap {
		fmt.Fprintln(file, product.HTMLString())
	}
}

func processResponse(resp *[]byte, recommendMap map[string]Product) []string {
	nextRequests := []string{}
	doc, err := html.Parse(bytes.NewReader(*resp))
	if err != nil {
		log.Fatal(err)
	}

	for n := range doc.Descendants() {
		parseProductBox(n, recommendMap, &nextRequests)
	}
	return nextRequests
}

func fetchRecommendations(url string) (*[]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching recommendations: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &body, nil
}

type Product struct {
	Id    string
	Title string
	Price string
	Link  string
	Image string
}

func (p Product) RecomString() string {
	return "https://arbysshop.com/recommendations/products?&section_id=product-recommendations&product_id=" + p.Id
}

func (p Product) HTMLString() string {
	return fmt.Sprintf(`<div class="product">
	<a href="%s">
		<img src="%s" alt="%s"/>
	</a>
	<h2>%s</h2>
	<p class="price">%s</p>
</div>`, p.Link, p.Image, p.Title, p.Title, p.Price)
}

func parseProductBox(n *html.Node, recommendMap map[string]Product, nextRequests *[]string) {
	if n.Type == html.ElementNode && n.DataAtom == atom.Div {
		if hasClass(n, "box") && hasClass(n, "product") {
			parseProduct(n, recommendMap, nextRequests)
		}
	}
}

func hasClass(n *html.Node, className string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			classes := strings.Fields(attr.Val)
			for _, class := range classes {
				if class == className {
					return true
				}
			}
		}
	}
	return false
}

func getAttribute(n *html.Node, attrName string) string {
	for _, attr := range n.Attr {
		if attr.Key == attrName {
			return attr.Val
		}
	}
	return ""
}

func getTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}

	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text.WriteString(getTextContent(c))
	}
	return strings.TrimSpace(text.String())
}

func parseProduct(productDiv *html.Node, recommendMap map[string]Product, nextRequests *[]string) {
	product := Product{}

	for n := range productDiv.Descendants() {
		findProductElements(n, &product)
	}

	recommendMap[product.Id] = product
	*nextRequests = append(*nextRequests, product.RecomString())
}

func findProductElements(n *html.Node, product *Product) {
	if n.Type == html.ElementNode {
		// Look for product card link (contains image and main link)
		if n.DataAtom == atom.A && hasClass(n, "product_card") {
			product.Link = "https://arbysshop.com" + getAttribute(n, "href")
			product.Id = regexp.MustCompile(`ProductGridImageWrapper-product-recommendations--(\d+)`).FindStringSubmatch(getAttribute(n, "id"))[1]
		}

		// Look for price span
		if n.DataAtom == atom.Span && hasClass(n, "price") {
			product.Price = strings.TrimSpace(getTextContent(n))
		}

		if n.DataAtom == atom.Img && hasClass(n, "product_card__image") && getAttribute(n, "data-fallback") != "" {
			product.Image = "https:" + getAttribute(n, "data-fallback")
			product.Title = getAttribute(n, "alt")
		}
	}
}
