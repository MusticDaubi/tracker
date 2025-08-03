package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

type Transaction struct {
	ID          int
	Type        string
	Category    string
	Amount      float64
	Description string
	Date        string
}

func main() {
	if runtime.GOOS == "windows" {
		enableANSISupport()
	}

	if err := InitDB(); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer db.Close()

	addCmd := flag.NewFlagSet("add", flag.ExitOnError)
	addType := addCmd.String("type", "", "Transaction type (income/expense)")
	addCategory := addCmd.String("category", "", "Category")
	addAmount := addCmd.Float64("amount", 0, "Amount")
	addDesc := addCmd.String("desc", "", "Description")
	addDate := addCmd.String("date", "", "Date (YYYY-MM-DD)")

	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	listType := listCmd.String("type", "", "Filter by type (income/expense)")
	listCategory := listCmd.String("category", "", "Filter by category")
	listStartDate := listCmd.String("start", "", "Start date (YYYY-MM-DD)")
	listEndDate := listCmd.String("end", "", "End date (YYYY-MM-DD)")
	listLimit := listCmd.Int("limit", 0, "Limit number of results")

	updateCmd := flag.NewFlagSet("update", flag.ExitOnError)
	updateID := updateCmd.Int("id", 0, "Transaction ID to update")
	updateType := updateCmd.String("type", "", "New transaction type")
	updateCategory := updateCmd.String("category", "", "New category")
	updateAmount := updateCmd.Float64("amount", -1, "New amount (use -1 to keep unchanged)")
	updateDesc := updateCmd.String("desc", "", "New description")
	updateDate := updateCmd.String("date", "", "New date (YYYY-MM-DD)")

	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	deleteID := deleteCmd.Int("id", 0, "Transaction ID to delete")

	statsCmd := flag.NewFlagSet("stats", flag.ExitOnError)
	statsPeriod := statsCmd.String("period", "all", "Time period (day/week/month/year/all)")
	statsStartDate := statsCmd.String("start", "", "Custom start date (YYYY-MM-DD)")
	statsEndDate := statsCmd.String("end", "", "Custom end date (YYYY-MM-DD)")

	if len(os.Args) < 2 {
		fmt.Println("Expected command: add, list, update, delete, stats")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		err := addCmd.Parse(os.Args[2:])
		if err != nil {
			return
		}
		transaction := Transaction{
			Type:        *addType,
			Category:    *addCategory,
			Amount:      *addAmount,
			Description: *addDesc,
			Date:        *addDate,
		}
		if err = validateTransaction(transaction); err != nil {
			log.Fatal("Validation error: ", err)
		}
		if err = AddTransaction(transaction); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Transaction added successfully!")

	case "list":
		err := listCmd.Parse(os.Args[2:])
		if err != nil {
			return
		}
		transactions, err := GetTransactions(
			*listType,
			*listCategory,
			*listStartDate,
			*listEndDate,
			*listLimit,
		)
		if err != nil {
			log.Fatal(err)
		}
		printTransactions(transactions)

	case "update":
		err := updateCmd.Parse(os.Args[2:])
		if err != nil {
			return
		}
		if *updateID == 0 {
			log.Fatal("Error: Transaction ID is required")
		}

		update := Transaction{
			Type:        *updateType,
			Category:    *updateCategory,
			Amount:      *updateAmount,
			Description: *updateDesc,
			Date:        *updateDate,
		}

		if err = UpdateTransaction(*updateID, update); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Transaction #%d updated successfully!\n", *updateID)

	case "delete":
		err := deleteCmd.Parse(os.Args[2:])
		if err != nil {
			return
		}
		if *deleteID == 0 {
			log.Fatal("Error: Transaction ID is required")
		}
		if err = DeleteTransaction(*deleteID); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Transaction #%d deleted successfully!\n", *deleteID)

	case "stats":
		err := statsCmd.Parse(os.Args[2:])
		if err != nil {
			return
		}
		income, expense, err := GetBalance(
			*statsPeriod,
			*statsStartDate,
			*statsEndDate,
		)
		if err != nil {
			log.Fatal(err)
		}

		stats, err := GetCategoryStats(
			*statsPeriod,
			*statsStartDate,
			*statsEndDate,
		)
		if err != nil {
			log.Fatal(err)
		}

		printStatistics(income, expense, stats)
	default:
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`Personal Finance Tracker - Usage:
  add     - Add new transaction
  list    - List transactions
  update  - Update transaction
  delete  - Delete transaction
  stats   - Show statistics

Use 'finance [command] -h' for command-specific help`)
}

func validateTransaction(t Transaction) error {
	if t.Type != "income" && t.Type != "expense" {
		return errors.New("type must be 'income' or 'expense'")
	}
	if t.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if t.Category == "" {
		return errors.New("category is required")
	}
	if _, err := time.Parse("2006-01-02", t.Date); err != nil {
		return errors.New("invalid date format, use YYYY-MM-DD")
	}
	return nil
}

func printTransactions(transactions []Transaction) {
	fmt.Printf("%-4s %-10s %-15s %-10s %-20s %-10s\n",
		"ID", "Date", "Type", "Amount", "Category", "Description")
	fmt.Println(strings.Repeat("-", 70))

	for _, t := range transactions {
		amountSign := ""
		if t.Type == "expense" {
			amountSign = "-"
		}
		fmt.Printf("%-4d %-10s %-15s %s%-9.2f %-20s %-10s\n",
			t.ID,
			t.Date,
			t.Type,
			amountSign,
			t.Amount,
			t.Category,
			t.Description)
	}
}

func printStatistics(income, expense float64, stats map[string]float64) {
	balance := income - expense

	const (
		reset  = "\033[0m"
		red    = "\033[31m"
		green  = "\033[32m"
		yellow = "\033[33m"
		cyan   = "\033[36m"
		bold   = "\033[1m"
	)

	fmt.Printf("\n%s=== FINANCIAL STATISTICS ===%s\n", bold, reset)

	fmt.Printf("\n%sTotal Income:%s  $%.2f\n", bold, reset, income)
	fmt.Printf("%sTotal Expenses:%s $%.2f\n", bold, reset, expense)

	balanceColor := green
	balanceSign := ""
	if balance < 0 {
		balanceColor = red
		balanceSign = "-"
	}
	fmt.Printf("%sBalance:%s       %s$%s%.2f%s\n",
		bold, reset, balanceColor, balanceSign, math.Abs(balance), reset)

	if len(stats) > 0 {
		fmt.Printf("\n%sExpenses by Category:%s\n", bold, reset)

		type CategoryStat struct {
			Name  string
			Value float64
		}

		var sortedStats []CategoryStat
		for category, amount := range stats {
			sortedStats = append(sortedStats, CategoryStat{category, amount})
		}

		sort.Slice(sortedStats, func(i, j int) bool {
			return sortedStats[i].Value > sortedStats[j].Value
		})

		totalExpense := expense
		if totalExpense == 0 {
			totalExpense = 1
		}

		for _, stat := range sortedStats {
			percentage := (stat.Value / totalExpense) * 100
			fmt.Printf(" - %s%-20s%s: $%s%.2f%s (%s%.1f%%%s)\n",
				cyan, stat.Name, reset,
				yellow, stat.Value, reset,
				green, percentage, reset)
		}

		topCount := 3
		if len(sortedStats) < 3 {
			topCount = len(sortedStats)
		}

		if topCount > 0 {
			fmt.Printf("\n%sTop %d Expenses:%s\n", bold, topCount, reset)
			for i := 0; i < topCount; i++ {
				fmt.Printf("%d. %s%s%s ($%s%.2f%s)\n",
					i+1,
					cyan, sortedStats[i].Name, reset,
					yellow, sortedStats[i].Value, reset)
			}
		}
	} else {
		fmt.Printf("\n%sNo expense data available%s\n", yellow, reset)
	}

	if expense > 0 && income > 0 {
		expenseRatio := expense / income
		fmt.Printf("\n%sExpense/Income Ratio:%s ", bold, reset)
		printProgressBar(expenseRatio)
	}
	fmt.Printf("%sExpense Categories:%s %d\n", bold, reset, len(stats))
	fmt.Println()
}

func printProgressBar(ratio float64) {
	const barWidth = 30
	filled := int(math.Round(ratio * barWidth))

	fmt.Print("[")
	for i := 0; i < barWidth; i++ {
		if i < filled {
			fmt.Print("█")
		} else {
			fmt.Print(" ")
		}
	}
	fmt.Printf("] %.1f%%\n", ratio*100)
}
func enableANSISupport() {
	enableForHandle(os.Stdout)
	enableForHandle(os.Stderr)
}

func enableForHandle(f *os.File) {
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return
	}
	const enableVirtualTerminalProcessing uint32 = 0x0004
	windows.SetConsoleMode(handle, mode|enableVirtualTerminalProcessing)
}
