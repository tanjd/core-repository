ALTER TABLE loan_requests ADD COLUMN returned_by INTEGER REFERENCES users(id);
