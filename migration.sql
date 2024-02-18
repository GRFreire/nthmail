CREATE TABLE inboxes (
    id integer not null primary key,
    addr text unique
);

CREATE TABLE mails (
    id integer not null primary key,
    inbox_id id,
    from_addr text,
    data blob,
    FOREIGN KEY(inbox_id) REFERENCES inboxes(id)
);
