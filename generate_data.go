package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	file, err := os.Create("generate_data.bat")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	budgets := []struct {
		category string
		amount   float64
		period   string
	}{
		{"food", 800, "monthly"},
		{"transport", 500, "monthly"},
		{"entertainment", 300, "monthly"},
		{"utilities", 400, "monthly"},
		{"health", 250, "monthly"},
		{"rent", 1200, "monthly"},
		{"shopping", 350, "monthly"},
		{"education", 200, "monthly"},
	}

	for _, b := range budgets {
		cmd := fmt.Sprintf(".\\finance.exe budget -add -category %s -amount %.2f -period %s\n",
			b.category, b.amount, b.period)
		file.WriteString(cmd)
	}

	// Получаем текущую дату
	now := time.Now()

	// Генерируем данные за последние 6 месяцев
	for monthOffset := 0; monthOffset < 6; monthOffset++ {
		// Вычисляем дату для текущего месяца в цикле
		currentDate := now.AddDate(0, -monthOffset, 0)
		year, month := currentDate.Year(), currentDate.Month()

		incomeCount := 2 + rand.Intn(2)
		for i := 0; i < incomeCount; i++ {
			amount := rand.Float64()*2000 + 1500
			categories := []string{"salary", "freelance", "investment", "bonus"}
			category := categories[rand.Intn(len(categories))]
			day := 1 + rand.Intn(5)
			date := fmt.Sprintf("%d-%02d-%02d", year, month, day)

			cmd := fmt.Sprintf(".\\finance.exe add -type income -category %s -amount %.2f -desc \"Income %d\" -date %s\n",
				category, amount, i+1, date)

			file.WriteString(cmd)
		}

		expenseCount := 15 + rand.Intn(10)
		for i := 0; i < expenseCount; i++ {
			categoryIdx := rand.Intn(len(budgets))
			category := budgets[categoryIdx].category

			maxAmount := budgets[categoryIdx].amount * 0.3
			amount := rand.Float64()*maxAmount + 10

			day := 1 + rand.Intn(28)
			date := fmt.Sprintf("%d-%02d-%02d", year, month, day)

			descriptions := map[string][]string{
				"food":          {"Groceries", "Restaurant", "Coffee", "Lunch", "Dinner"},
				"transport":     {"Bus fare", "Taxi", "Gas", "Metro", "Parking"},
				"entertainment": {"Cinema", "Concert", "Netflix", "Books", "Games"},
				"utilities":     {"Electricity", "Water", "Internet", "Phone"},
				"health":        {"Doctor", "Medicine", "Gym", "Vitamins"},
				"rent":          {"Rent payment"},
				"shopping":      {"Clothes", "Electronics", "Furniture"},
				"education":     {"Courses", "Books", "Seminar"},
			}

			desc := descriptions[category][rand.Intn(len(descriptions[category]))]

			cmd := fmt.Sprintf(".\\finance.exe add -type expense -category %s -amount %.2f -desc \"%s\" -date %s\n",
				category, amount, desc, date)

			file.WriteString(cmd)
		}
	}

	fmt.Println("Generated comprehensive test data with budgets and transactions for the last 6 months")
	fmt.Println("Run 'generate_data.bat' to populate the database")
}
