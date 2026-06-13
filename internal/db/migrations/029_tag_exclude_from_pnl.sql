-- +goose Up
-- Add flag to exclude tags from P&L reports (for balance sheet movements like transfers)

ALTER TABLE tags ADD COLUMN exclude_from_pnl BOOLEAN NOT NULL DEFAULT false;

-- Mark known balance-sheet-movement tags as excluded from P&L
UPDATE tags SET exclude_from_pnl = true 
WHERE name IN (
    'Internal Transfer',
    'Credit Card Payment',
    'Credit Card Payments', 
    'Investment Transfer',
    'Investment Transfers',
    'Transfers',
    'Bank Transfer',
    'Bank Transfers',
    'Account Transfer',
    'Account Transfers',
    'Loan Payments'
);

-- +goose Down
ALTER TABLE tags DROP COLUMN exclude_from_pnl;
