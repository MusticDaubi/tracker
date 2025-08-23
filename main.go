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

type Budget struct {
	ID        int
	Category  string
	Amount    float64
	Period    string
	StartDate string
	EndDate   string
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

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

	budgetCmd := flag.NewFlagSet("budget", flag.ExitOnError)
	budgetAdd := budgetCmd.Bool("add", false, "Add new budget")
	budgetList := budgetCmd.Bool("list", false, "List all budgets")
	budgetRemove := budgetCmd.Bool("remove", false, "Remove budget")
	budgetCategory := budgetCmd.String("category", "", "Budget category")
	budgetAmount := budgetCmd.Float64("amount", 0, "Budget amount")
	budgetPeriod := budgetCmd.String("period", "monthly", "Budget period (monthly/weekly/yearly)")
	budgetStart := budgetCmd.String("start", "", "Start date (YYYY-MM-DD)")
	budgetEnd := budgetCmd.String("end", "", "End date (YYYY-MM-DD)")

	resetCmd := flag.NewFlagSet("reset", flag.ExitOnError)
	resetConfirm := resetCmd.Bool("confirm", false, "Confirm database reset")

	if len(os.Args) < 2 {
		printHelp()
		fmt.Println("\nThe application will now close.")
		fmt.Println("To use the application, open a command prompt and run:")
		fmt.Println("finance.exe [command] [flags]")
		fmt.Println("\nPress Enter to exit...")
		fmt.Scanln()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		err := addCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Printf("Error: %s \n", err)
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
		if transaction.Type == "expense" {
			spent, total, err := CheckBudget(transaction.Category, "monthly")
			if err == nil {
				percentage := (spent / total) * 100
				if percentage > 100 {
					fmt.Printf("%sWARNING: Budget exceeded for %s! (%.1f%%)%s\n",
						colorRed, transaction.Category, percentage, colorReset)
				} else if percentage > 90 {
					fmt.Printf("%sWARNING: Approaching budget limit for %s (%.1f%%)%s\n",
						colorYellow, transaction.Category, percentage, colorReset)
				}
			}
		}
		fmt.Println("Transaction added successfully!")
	case "reset":
		err := resetCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Printf("Error: %s \n", err)
			return
		}
		if !*resetConfirm {
			fmt.Println("Warning: This will delete all data! Use -confirm to proceed")
			return
		}
		if err = ResetDB(); err != nil {
			log.Fatal("Reset error: ", err)
		}
		fmt.Println("Database reset successfully")

	case "list":
		err := listCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Printf("Error: %s \n", err)
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
			fmt.Printf("Error: %s \n", err)
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
			fmt.Printf("Error: %s \n", err)
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
			fmt.Printf("Error: %s \n", err)
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
	case "budget":
		err := budgetCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Printf("Error: %s \n", err)
			return
		}
		if *budgetAdd {
			if *budgetCategory == "" || *budgetAmount <= 0 {
				log.Fatal("Category and amount are required")
			}
			budget := Budget{
				Category:  *budgetCategory,
				Amount:    *budgetAmount,
				Period:    *budgetPeriod,
				StartDate: *budgetStart,
				EndDate:   *budgetEnd,
			}

			if err = validateBudget(budget); err != nil {
				log.Fatal("Budget validation error: ", err)
			}

			if err = AddBudget(budget); err != nil {
				log.Fatal(err)
			}

			fmt.Println("Budget added successfully!")
		} else if *budgetList {
			budgets, err := GetBudgets()
			if err != nil {
				log.Fatal(err)
			}
			printBudgets(budgets)
		} else if *budgetRemove {
			if *budgetCategory == "" {
				log.Fatal("Category is required")
			}
			if err = RemoveBudget(*budgetCategory); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Budget for category '%s' removed\n", *budgetCategory)
		} else {
			budgetCmd.Usage()
		}
	default:
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`Personal Finance Tracker - Usage:
    
Commands:
  add     - Add new transaction
  list    - List transactions
  update  - Update transaction
  delete  - Delete transaction
  stats   - Show statistics
  budget  - Manage budgets
  reset   - Reset database

Examples:
  finance add -type income -category salary -amount 2500 -date 2023-09-01
  finance list -type expense
  finance stats -period month

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
	useColor := isColorSupported()

	reset, red, green, yellow, cyan, bold := "", "", "", "", "", ""
	if useColor {
		reset = "\033[0m"
		red = "\033[31m"
		green = "\033[32m"
		yellow = "\033[33m"
		cyan = "\033[36m"
		bold = "\033[1m"
	}

	fmt.Printf("\n%s=== FINANCIAL STATISTICS ===%s\n", bold, reset)

	fmt.Printf("\n%sTotal Income:%s  $%.2f\n", bold, reset, income)
	fmt.Printf("%sTotal Expenses:%s $%.2f\n", bold, reset, expense)

	budgets, err := GetBudgets()
	if err == nil && len(budgets) > 0 {
		fmt.Printf("\n%sBudget Status:%s\n", bold, reset)

		for _, budget := range budgets {
			spent, total, err := CheckBudget(budget.Category, budget.Period)
			if err != nil {
				continue
			}

			percentage := (spent / total) * 100
			statusColor := green
			if percentage > 90 {
				statusColor = red
			} else if percentage > 75 {
				statusColor = yellow
			}

			fmt.Printf(" - %s%-15s%s: $%s%.2f%s / $%s%.2f%s (%s%.1f%%%s)\n",
				cyan, budget.Category, reset,
				statusColor, spent, reset,
				yellow, total, reset,
				statusColor, percentage, reset)
		}
	}

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

func isColorSupported() bool {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}

	if runtime.GOOS == "windows" {
		return true
	}

	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		return true
	}

	return false
}

func printBudgets(budgets []Budget) {
	useColor := isColorSupported()
	reset, bold := "", ""
	if useColor {
		reset = colorReset
		bold = colorBold
	}

	fmt.Printf("\n%s=== BUDGETS ===%s\n", bold, reset)
	fmt.Printf("%-4s %-15s %-10s %-10s %-12s %-12s\n", "ID", "Category", "Amount", "Period", "Start", "End")
	fmt.Println(strings.Repeat("-", 65))

	for _, b := range budgets {
		fmt.Printf("%-4d %-15s $%-9.2f %-10s %-12s %-12s\n",
			b.ID,
			b.Category,
			b.Amount,
			b.Period,
			b.StartDate,
			b.EndDate)
	}
}

func validateBudget(b Budget) error {
	if b.Category == "" {
		return errors.New("category is required")
	}

	if b.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	validPeriods := map[string]bool{
		"monthly": true,
		"weekly":  true,
		"yearly":  true,
	}
	if !validPeriods[b.Period] {
		return errors.New("invalid period, must be monthly, weekly or yearly")
	}

	if b.StartDate != "" {
		if _, err := time.Parse("2006-01-02", b.StartDate); err != nil {
			return errors.New("invalid start date format, use YYYY-MM-DD")
		}
	}

	if b.EndDate != "" {
		if _, err := time.Parse("2006-01-02", b.EndDate); err != nil {
			return errors.New("invalid end date format, use YYYY-MM-DD")
		}
	}

	if b.EndDate != "" && b.StartDate == "" {
		return errors.New("start date is required when end date is specified")
	}

	if b.StartDate != "" && b.EndDate != "" {
		start, _ := time.Parse("2006-01-02", b.StartDate)
		end, _ := time.Parse("2006-01-02", b.EndDate)
		if end.Before(start) {
			return errors.New("end date cannot be before start date")
		}
	}

	return nil
}
