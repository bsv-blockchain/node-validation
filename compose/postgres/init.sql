-- compose/postgres/init.sql
CREATE USER teranode WITH PASSWORD 'teranode';

CREATE DATABASE teranode1 OWNER teranode;
CREATE DATABASE teranode2 OWNER teranode;
CREATE DATABASE teranode3 OWNER teranode;

GRANT ALL PRIVILEGES ON DATABASE teranode1 TO teranode;
GRANT ALL PRIVILEGES ON DATABASE teranode2 TO teranode;
GRANT ALL PRIVILEGES ON DATABASE teranode3 TO teranode;
