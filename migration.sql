pragma journal_mode = wal;

CREATE TABLE mails (
    id integer not null primary key,
    arrived_at integer not null,
    rcpt_addr text not null,
    from_addr text not null,
    data blob not null
);
