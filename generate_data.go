package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	file, _ := os.Create("generate.bat")
	defer file.Close()

	for i := 0; i < 100; i++ {
		ttype := "expense"
		if i%5 == 0 {
			ttype = "income"
		}

		categories := []string{"food", "transport", "entertainment", "utilities", "health"}
		category := categories[rand.Intn(len(categories))]

		amount := rand.Float64()*500 + 10
		date := fmt.Sprintf("2023-%02d-%02d", rand.Intn(12)+1, rand.Intn(28)+1)

		cmd := fmt.Sprintf("finance add -type %s -category %s -amount %.2f -desc \"Transaction %d\" -date %s\n",
			ttype, category, amount, i+1, date)

		file.WriteString(cmd)
	}
}
