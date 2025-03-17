#!/bin/bash

# Set variable for tablespace path
TABLESPACE_PATH="/Volumes/DATA/db/health/health_datafile.ibd"
USERS="root"

# Create a temporary file for the SQL script
TEMP_SQL_FILE=$(mktemp)

# Replace path in SQL file
sed "s|/Volumes/DATA/db/health/my_health_datafile.ibd|$TABLESPACE_PATH|g" create_db_health.sql > $TEMP_SQL_FILE
#cat $TEMP_SQL_FILE

# Execute SQL script with mysqlsh
mysqlsh --user=$USERS --password --host=localhost --sql < $TEMP_SQL_FILE

# Supprimer le fichier temporaire
rm $TEMP_SQL_FILE

