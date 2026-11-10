-- Data-loss note: drops every registered mobile push device and pending push ticket.
DROP TABLE IF EXISTS push_tickets;
DROP TABLE IF EXISTS push_devices;
