# Convert Expenses Amounts to EUR Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a database migration to convert all expense amounts from BGN to EUR using the fixed rate of 0.51130.

**Architecture:** A new Goose migration file will be added to the `server/internal/db/migrations` directory.

**Tech Stack:** SQL (PostgreSQL), Goose migrations.

---

### Task 1: Create Migration File

**Files:**
- Create: `server/internal/db/migrations/20260513100000_convert_amounts_to_eur.sql`

- [ ] **Step 1: Create the migration file with Up and Down logic**

```sql
-- +goose Up
-- Convert amounts from BGN to EUR using RoE 0.51130
UPDATE car_expenses SET amount = ROUND(amount * 0.51130, 2);
UPDATE home_expenses SET amount = ROUND(amount * 0.51130, 2);

-- +goose Down
-- Convert amounts back from EUR to BGN using RoE 0.51130
UPDATE car_expenses SET amount = ROUND(amount / 0.51130, 2);
UPDATE home_expenses SET amount = ROUND(amount / 0.51130, 2);
```

- [ ] **Step 2: Verify the file exists and has correct content**

Run: `ls server/internal/db/migrations/20260513100000_convert_amounts_to_eur.sql`
Expected: File exists.

- [ ] **Step 3: Commit the migration**

```bash
git add server/internal/db/migrations/20260513100000_convert_amounts_to_eur.sql
git commit -m "db: add migration to convert expense amounts to EUR"
```
