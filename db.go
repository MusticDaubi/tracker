package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB() error {
	var err error
	db, err = sql.Open("sqlite", "./finance.db")
	if err != nil {
		return err
	}

	createTable := `
    CREATE TABLE IF NOT EXISTS transactions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        type TEXT NOT NULL CHECK(type IN ('income', 'expense')),
        category TEXT NOT NULL,
        amount REAL NOT NULL,
        description TEXT,
        date TEXT NOT NULL
    );
    `

	_, err = db.Exec(createTable)
	if err != nil {
		return err
	}
	createIndexes := `
    CREATE INDEX IF NOT EXISTS idx_type ON transactions(type);
    CREATE INDEX IF NOT EXISTS idx_date ON transactions(date);
    CREATE INDEX IF NOT EXISTS idx_category ON transactions(category);
    `

	_, err = db.Exec(createIndexes)
	if err != nil {
		return err
	}

	createBudgetTable := `
    CREATE TABLE IF NOT EXISTS budgets (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        category TEXT NOT NULL UNIQUE,
        amount REAL NOT NULL,
        period TEXT NOT NULL,
        start_date TEXT,
        end_date TEXT
    );`

	_, err = db.Exec(createBudgetTable)

	return err
}

func ResetDB() error {
	_, err := db.Exec("PRAGMA foreign_keys = OFF")
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	tables := []string{"transactions", "budgets"}
	for _, table := range tables {
		_, err = tx.Exec("DELETE FROM " + table)
		if err != nil {
			tx.Rollback()
			return err
		}

		_, err = tx.Exec("DELETE FROM sqlite_sequence WHERE name = ?", table)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	_, err = tx.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func AddBudget(b Budget) error {
	existing, err := GetBudget(b.Category)
	if err == nil && existing.ID != 0 {
		return errors.New("budget for this category already exists")
	}

	query := `
        INSERT OR REPLACE INTO budgets (category, amount, period, start_date, end_date)
        VALUES (:category, :amount, :period, :start_date, :end_date)
    `
	_, err = db.Exec(query, sql.Named("category", b.Category), sql.Named("amount", b.Amount), sql.Named("period", b.Period), sql.Named("start_date", b.StartDate), sql.Named("end_date", b.EndDate))
	return err
}

func GetBudgets() ([]Budget, error) {
	rows, err := db.Query("SELECT id, category, amount, period, start_date, end_date FROM budgets")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var budgets []Budget
	for rows.Next() {
		var b Budget
		err = rows.Scan(&b.ID, &b.Category, &b.Amount, &b.Period, &b.StartDate, &b.EndDate)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, nil
}

func GetBudget(category string) (Budget, error) {
	var b Budget
	row := db.QueryRow("SELECT id, category, amount, period, start_date, end_date FROM budgets WHERE category = ?", category)
	err := row.Scan(&b.ID, &b.Category, &b.Amount, &b.Period, &b.StartDate, &b.EndDate)
	return b, err
}

func RemoveBudget(category string) error {
	_, err := db.Exec("DELETE FROM budgets WHERE category = ?", category)
	return err
}

func CheckBudget(category string, period string) (currentSpent, budgetAmount float64, err error) {
	b, err := GetBudget(category)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	var whereClause string
	switch period {
	case "monthly":
		whereClause = "strftime('%Y-%m', date) = strftime('%Y-%m', 'now')"
	case "weekly":
		whereClause = "date >= date('now', 'weekday 0', '-7 days') AND date <= date('now')"
	case "yearly":
		whereClause = "strftime('%Y', date) = strftime('%Y', 'now')"
	default:
		whereClause = "1=1"
	}

	query := fmt.Sprintf(`
        SELECT COALESCE(SUM(amount), 0)
        FROM transactions
        WHERE type = 'expense' AND category = ? AND %s
    `, whereClause)

	row := db.QueryRow(query, category)
	err = row.Scan(&currentSpent)
	if err != nil {
		return 0, 0, err
	}

	return currentSpent, b.Amount, nil
}

func AddTransaction(t Transaction) error {
	query := `
        INSERT INTO transactions (type, category, amount, description, date)
        VALUES (:type, :category, :amount, :description, :date)
        `
	_, err := db.Exec(query, sql.Named("type", t.Type), sql.Named("category", t.Category), sql.Named("amount", t.Amount), sql.Named("description", t.Description), sql.Named("date", t.Date))
	return err
}

func GetTransactions(tType, category, startDate, endDate string, limit int) ([]Transaction, error) {
	query := "SELECT id, type, category, amount, description, date FROM transactions"
	var conditions []string
	var args []interface{}

	if tType != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, tType)
	}
	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}
	if startDate != "" {
		conditions = append(conditions, "date >= ?")
		args = append(args, startDate)
	}
	if endDate != "" {
		conditions = append(conditions, "date <= ?")
		args = append(args, endDate)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY date DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		err = rows.Scan(&t.ID, &t.Type, &t.Category, &t.Amount, &t.Description, &t.Date)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func UpdateTransaction(id int, t Transaction) error {
	query := "UPDATE transactions SET "
	var updates []string
	var args []interface{}

	if t.Type != "" {
		updates = append(updates, "type = ?")
		args = append(args, t.Type)
	}
	if t.Category != "" {
		updates = append(updates, "category = ?")
		args = append(args, t.Category)
	}
	if t.Amount >= 0 {
		updates = append(updates, "amount = ?")
		args = append(args, t.Amount)
	}
	if t.Description != "" {
		updates = append(updates, "description = ?")
		args = append(args, t.Description)
	}
	if t.Date != "" {
		updates = append(updates, "date = ?")
		args = append(args, t.Date)
	}

	if len(updates) == 0 {
		return errors.New("nothing to update")
	}

	query += strings.Join(updates, ", ")
	query += " WHERE id = ?"
	args = append(args, id)

	_, err := db.Exec(query, args...)
	return err
}

func DeleteTransaction(id int) error {
	query := `DELETE FROM transactions WHERE id = :id`
	_, err := db.Exec(query, sql.Named("id", id))
	return err
}

func GetBalance(period, startDate, endDate string) (income, expense float64, err error) {
	var whereClause string
	var args []interface{}

	switch period {
	case "day":
		whereClause = "date = date('now')"
	case "week":
		whereClause = "date >= date('now', 'weekday 0', '-7 days') AND date <= date('now')"
	case "month":
		whereClause = "strftime('%Y-%m', date) = strftime('%Y-%m', 'now')"
	case "year":
		whereClause = "strftime('%Y', date) = strftime('%Y', 'now')"
	case "custom":
		if startDate == "" || endDate == "" {
			return 0, 0, errors.New("start and end dates required for custom period")
		}
		whereClause = "date BETWEEN ? AND ?"
		args = append(args, startDate, endDate)
	default:
		whereClause = "1=1"
	}

	query := fmt.Sprintf(`
        SELECT COALESCE(SUM(amount), 0)
        FROM transactions
        WHERE type = 'income' AND %s`, whereClause)
	row := db.QueryRow(query, args...)
	err = row.Scan(&income)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get income: %w", err)
	}

	query = fmt.Sprintf(`
        SELECT COALESCE(SUM(amount), 0)
        FROM transactions
        WHERE type = 'expense' AND %s`, whereClause)
	row = db.QueryRow(query, args...)
	err = row.Scan(&expense)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get expense: %w", err)
	}

	return income, expense, nil

}

func GetCategoryStats(period, startDate, endDate string) (map[string]float64, error) {
	stats := make(map[string]float64)

	query := `
        SELECT category, SUM(amount) 
        FROM transactions 
        WHERE type = 'expense'
    `

	var args []interface{}
	var conditions []string

	switch period {
	case "day":
		conditions = append(conditions, "date = date('now')")
	case "week":
		conditions = append(conditions, "date >= date('now', 'weekday 0', '-7 days')")
		conditions = append(conditions, "date <= date('now')")
	case "month":
		conditions = append(conditions, "strftime('%Y-%m', date) = strftime('%Y-%m', 'now')")
	case "year":
		conditions = append(conditions, "strftime('%Y', date) = strftime('%Y', 'now')")
	case "custom":
		if startDate == "" || endDate == "" {
			return nil, errors.New("start and end dates required for custom period")
		}
		conditions = append(conditions, "date BETWEEN ? AND ?")
		args = append(args, startDate, endDate)
	}

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	query += " GROUP BY category"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var category string
		var total sql.NullFloat64
		err = rows.Scan(&category, &total)
		if err != nil {
			return nil, err
		}
		if total.Valid {
			stats[category] = total.Float64
		} else {
			stats[category] = 0
		}
	}
	return stats, nil
}
