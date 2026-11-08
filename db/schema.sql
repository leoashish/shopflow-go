-- ShopFlow schema (SQLite). Money is ALWAYS stored as integer cents -- never floating point.

CREATE TABLE products (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    sku          TEXT    NOT NULL UNIQUE,
    name         TEXT    NOT NULL,
    price_cents  INTEGER NOT NULL CHECK (price_cents >= 0),
    stock        INTEGER NOT NULL CHECK (stock >= 0),
    category     TEXT    NOT NULL
);

CREATE TABLE customers (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT    NOT NULL,
    email              TEXT    NOT NULL UNIQUE,
    tier               TEXT    NOT NULL CHECK (tier IN ('Standard', 'Silver', 'Gold')),
    is_active          INTEGER NOT NULL DEFAULT 1,
    credit_limit_cents INTEGER NOT NULL
);

CREATE TABLE orders (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    customer_id     INTEGER NOT NULL REFERENCES customers(id),
    total_cents     INTEGER NOT NULL,
    status          TEXT    NOT NULL CHECK (status IN ('Pending', 'Confirmed', 'Shipped', 'Cancelled')),
    created_at_utc  TEXT    NOT NULL
);

CREATE TABLE order_lines (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id          INTEGER NOT NULL REFERENCES orders(id),
    product_id        INTEGER NOT NULL REFERENCES products(id),
    quantity          INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents  INTEGER NOT NULL
);

-- A nightly data pipeline, used by the graph algorithms step (topological sort).
CREATE TABLE pipeline_tasks (
    name TEXT PRIMARY KEY
);

CREATE TABLE pipeline_dependencies (
    task       TEXT NOT NULL REFERENCES pipeline_tasks(name),
    depends_on TEXT NOT NULL REFERENCES pipeline_tasks(name),
    PRIMARY KEY (task, depends_on)
);
