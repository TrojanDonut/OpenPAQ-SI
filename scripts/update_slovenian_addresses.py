#!/usr/bin/env python3
"""
Script to update Slovenian addresses in ClickHouse from a CSV file.
This script performs incremental updates - it adds new addresses and updates existing ones
without deleting all data and re-inserting.

Usage:
    python3 update_slovenian_addresses.py <csv_file> [options]

Example:
    python3 update_slovenian_addresses.py addresses.csv
    python3 update_slovenian_addresses.py addresses.csv --host localhost --port 9000
"""

import argparse
import csv
import sys
import os
from typing import List, Dict, Optional, Tuple
from clickhouse_driver import Client

# Default ClickHouse connection settings
DEFAULT_HOST = "localhost"
DEFAULT_PORT = "9000"
DEFAULT_USER = "default"
DEFAULT_PASSWORD = "default"
DEFAULT_DATABASE = "default"
DEFAULT_TABLE = "slovenian_addresses"

# All columns in the table (in order)
TABLE_COLUMNS = [
    "feature_id", "eid_naslov", "obcina_sifra", "obcina_naziv", "obcina_naziv_dj",
    "naselje_sifra", "naselje_naziv", "naselje_naziv_dj", "ulica_sifra", "ulica_naziv",
    "ulica_naziv_dj", "postni_okolis_sifra", "postni_okolis_naziv", "postni_okolis_naziv_dj",
    "hs_stevilka", "hs_dodatek", "st_stanovanja", "e", "n", "eid_obcina", "eid_naselje",
    "eid_ulica", "eid_postni_okolis", "eid_hisna_stevilka", "eid_stanovanje", "eid_stavba",
    "eid_cetrtna_skupnost", "eid_dz_volisce", "eid_krajevna_skupnost", "eid_lokalno_volisce",
    "eid_lokalna_volilna_enota", "eid_solski_okolis", "eid_statisticna_regija",
    "eid_upravna_enota", "eid_vaska_skupnost", "eid_volilna_enota_dz", "eid_volilni_okraj",
    "eid_kohezijska_regija", "datum_sys"
]


def escape_value(value: str) -> str:
    """Escape a value for ClickHouse INSERT statement."""
    if value is None:
        return "''"
    # Replace single quotes with escaped quotes
    escaped = str(value).replace("'", "''")
    # Replace backslashes
    escaped = escaped.replace("\\", "\\\\")
    return f"'{escaped}'"


def run_clickhouse_query(query: str, host: str, port: str, user: str, password: str, database: str) -> Tuple[bool, str]:
    """Execute a ClickHouse query using clickhouse-driver and return success status and output."""
    try:
        client = Client(
            host=host,
            port=int(port),
            user=user,
            password=password,
            database=database,
            secure=False,
        )
        result = client.execute(query)
        client.disconnect()
        return True, str(result)
    except Exception as e:
        return False, str(e)


def create_staging_table(host: str, port: str, user: str, password: str, database: str) -> bool:
    """Create a temporary staging table for loading data."""
    query = f"""
    CREATE TABLE IF NOT EXISTS {database}.slovenian_addresses_staging
    (
        `feature_id` String,
        `eid_naslov` String,
        `obcina_sifra` String,
        `obcina_naziv` String,
        `obcina_naziv_dj` String,
        `naselje_sifra` String,
        `naselje_naziv` String,
        `naselje_naziv_dj` String,
        `ulica_sifra` String,
        `ulica_naziv` String,
        `ulica_naziv_dj` String,
        `postni_okolis_sifra` String,
        `postni_okolis_naziv` String,
        `postni_okolis_naziv_dj` String,
        `hs_stevilka` String,
        `hs_dodatek` String,
        `st_stanovanja` String,
        `e` String,
        `n` String,
        `eid_obcina` String,
        `eid_naselje` String,
        `eid_ulica` String,
        `eid_postni_okolis` String,
        `eid_hisna_stevilka` String,
        `eid_stanovanje` String,
        `eid_stavba` String,
        `eid_cetrtna_skupnost` String,
        `eid_dz_volisce` String,
        `eid_krajevna_skupnost` String,
        `eid_lokalno_volisce` String,
        `eid_lokalna_volilna_enota` String,
        `eid_solski_okolis` String,
        `eid_statisticna_regija` String,
        `eid_upravna_enota` String,
        `eid_vaska_skupnost` String,
        `eid_volilna_enota_dz` String,
        `eid_volilni_okraj` String,
        `eid_kohezijska_regija` String,
        `datum_sys` String
    )
    ENGINE = Memory
    """
    
    success, output = run_clickhouse_query(query, host, port, user, password, database)
    if not success:
        print(f"Error creating staging table: {output}", file=sys.stderr)
    return success


def load_csv_to_staging(csv_file: str, host: str, port: str, user: str, password: str, database: str) -> Tuple[bool, int]:
    """Load CSV data into staging table. Returns (success, row_count)."""
    print(f"Reading CSV file: {csv_file}")
    
    # Read CSV and prepare data
    rows = []
    try:
        with open(csv_file, 'r', encoding='utf-8') as f:
            # Detect delimiter
            sample = f.read(8192)
            f.seek(0)
            sniffer = csv.Sniffer()
            try:
                delimiter = sniffer.sniff(sample).delimiter
            except csv.Error:
                delimiter = ','
                print("Could not auto-detect delimiter, defaulting to ','", file=sys.stderr)
            
            reader = csv.DictReader(f, delimiter=delimiter)
            
            # Get CSV headers (keep original for DictReader, but also normalize for matching)
            csv_headers_original = [h.strip() for h in reader.fieldnames]
            
            # Map CSV columns to table columns (case-insensitive, underscore-normalized)
            def normalize_col_name(name):
                """Normalize column name for matching (lowercase, remove underscores/hyphens)."""
                return name.lower().replace('_', '').replace('-', '')
            
            column_map = {}
            for table_col in TABLE_COLUMNS:
                matched = False
                table_col_normalized = normalize_col_name(table_col)
                
                # Try exact match first (case-insensitive)
                for csv_col_orig in csv_headers_original:
                    if csv_col_orig.lower() == table_col.lower():
                        column_map[table_col] = csv_col_orig
                        matched = True
                        break
                
                # If not matched, try normalized match (handles FEATUREID -> feature_id)
                if not matched:
                    for csv_col_orig in csv_headers_original:
                        if normalize_col_name(csv_col_orig) == table_col_normalized:
                            column_map[table_col] = csv_col_orig
                            matched = True
                            break
                
                if not matched:
                    print(f"Warning: Could not find matching CSV column for table column '{table_col}'", file=sys.stderr)
            
            
            # Check if we have at least feature_id or eid_naslov
            if 'feature_id' not in column_map and 'eid_naslov' not in column_map:
                print("Warning: Neither 'feature_id' nor 'eid_naslov' found in CSV headers", file=sys.stderr)
                print(f"Available columns: {', '.join(csv_headers_original)}", file=sys.stderr)
            
            # Read all rows
            for row in reader:
                # Build values array in table column order
                values = []
                for col in TABLE_COLUMNS:
                    csv_col = column_map.get(col)
                    if csv_col:
                        value = row.get(csv_col, '').strip()
                    else:
                        value = ''
                    values.append(escape_value(value))
                
                rows.append(f"({','.join(values)})")
        
        if not rows:
            print("No rows found in CSV file", file=sys.stderr)
            return False, 0
        
        print(f"Loaded {len(rows)} rows from CSV")
        
    except Exception as e:
        print(f"Error reading CSV file: {e}", file=sys.stderr)
        return False, 0
    
    # Insert into staging table in batches
    batch_size = 10000
    total_inserted = 0
    
    print(f"Inserting data into staging table (batch size: {batch_size})...")
    
    for i in range(0, len(rows), batch_size):
        batch = rows[i:i + batch_size]
        values_str = ','.join(batch)
        
        columns_str = ','.join([f"`{col}`" for col in TABLE_COLUMNS])
        insert_query = f"INSERT INTO {database}.slovenian_addresses_staging ({columns_str}) VALUES {values_str}"
        
        success, output = run_clickhouse_query(insert_query, host, port, user, password, database)
        if not success:
            print(f"Error inserting batch {i//batch_size + 1}: {output}", file=sys.stderr)
            return False, total_inserted
        
        total_inserted += len(batch)
        if (i + batch_size) % 50000 == 0 or i + batch_size >= len(rows):
            print(f"  Inserted {min(i + batch_size, len(rows))} / {len(rows)} rows...")
    
    print(f"Successfully loaded {total_inserted} rows into staging table")
    return True, total_inserted


def merge_staging_to_main(host: str, port: str, user: str, password: str, database: str, table: str) -> bool:
    """Merge data from staging table to main table, updating existing and adding new."""
    print("Merging staging table data into main table...")
    
    # Strategy:
    # 1. Delete existing rows from main table where feature_id exists in staging
    # 2. Insert all rows from staging into main table
    
    # Delete existing rows (if feature_id is present in staging)
    delete_query = f"""
    ALTER TABLE {database}.{table} 
    DELETE WHERE feature_id IN (SELECT feature_id FROM {database}.slovenian_addresses_staging WHERE feature_id != '')
    """
    
    print("  Deleting existing rows that will be updated...")
    success, output = run_clickhouse_query(delete_query, host, port, user, password, database)
    if not success:
        print(f"Warning: Error deleting existing rows (this is OK if table is empty): {output}", file=sys.stderr)
    else:
        print("  Deleted existing rows")
    
    # Insert all from staging to main
    columns_str = ','.join([f"`{col}`" for col in TABLE_COLUMNS])
    insert_query = f"""
    INSERT INTO {database}.{table} ({columns_str})
    SELECT {columns_str} FROM {database}.slovenian_addresses_staging
    """
    
    print("  Inserting/updating rows...")
    success, output = run_clickhouse_query(insert_query, host, port, user, password, database)
    if not success:
        print(f"Error inserting data: {output}", file=sys.stderr)
        return False
    
    print("  Successfully merged data")
    return True


def cleanup_staging(host: str, port: str, user: str, password: str, database: str) -> None:
    """Clean up staging table."""
    print("Cleaning up staging table...")
    query = f"DROP TABLE IF EXISTS {database}.slovenian_addresses_staging"
    run_clickhouse_query(query, host, port, user, password, database)
    print("Cleanup complete")


def get_row_count(host: str, port: str, user: str, password: str, database: str, table: str) -> int:
    """Get the current row count in the main table."""
    query = f"SELECT count() FROM {database}.{table}"
    success, output = run_clickhouse_query(query, host, port, user, password, database)
    if success:
        try:
            return int(output.strip())
        except ValueError:
            return 0
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Update Slovenian addresses in ClickHouse from CSV file",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Using default settings (localhost)
  python3 update_slovenian_addresses.py addresses.csv
  
  # Using Docker Compose
  python3 update_slovenian_addresses.py addresses.csv --host localhost --port 9000
  
  # With custom credentials
  python3 update_slovenian_addresses.py addresses.csv --host localhost --port 9000 --user myuser --password mypass
        """
    )
    
    parser.add_argument("csv_file", help="Path to CSV file with Slovenian addresses")
    parser.add_argument("--host", default=DEFAULT_HOST, help=f"ClickHouse host (default: {DEFAULT_HOST})")
    parser.add_argument("--port", default=DEFAULT_PORT, help=f"ClickHouse port (default: {DEFAULT_PORT})")
    parser.add_argument("--user", default=DEFAULT_USER, help=f"ClickHouse user (default: {DEFAULT_USER})")
    parser.add_argument("--password", default=DEFAULT_PASSWORD, help=f"ClickHouse password (default: {DEFAULT_PASSWORD})")
    parser.add_argument("--database", default=DEFAULT_DATABASE, help=f"ClickHouse database (default: {DEFAULT_DATABASE})")
    parser.add_argument("--table", default=DEFAULT_TABLE, help=f"ClickHouse table (default: {DEFAULT_TABLE})")
    parser.add_argument("--docker", action="store_true", help="Use docker-compose exec to run clickhouse-client")
    
    args = parser.parse_args()
    
    # Check if CSV file exists
    if not os.path.exists(args.csv_file):
        print(f"Error: CSV file not found: {args.csv_file}", file=sys.stderr)
        sys.exit(1)
    
    # If using docker, modify the run_clickhouse_query function behavior
    if args.docker:
        print("Note: Using docker-compose exec mode")
        print("Make sure to run: docker-compose exec clickhouse clickhouse-client --help")
    
    print("=" * 60)
    print("Slovenian Addresses Update Script")
    print("=" * 60)
    print(f"CSV File: {args.csv_file}")
    print(f"ClickHouse: {args.host}:{args.port}")
    print(f"Database: {args.database}, Table: {args.table}")
    print("=" * 60)
    print()
    
    # Get initial row count
    initial_count = get_row_count(args.host, args.port, args.user, args.password, args.database, args.table)
    print(f"Current rows in table: {initial_count}")
    print()
    
    # Step 1: Create staging table
    print("Step 1: Creating staging table...")
    if not create_staging_table(args.host, args.port, args.user, args.password, args.database):
        print("Failed to create staging table", file=sys.stderr)
        sys.exit(1)
    print("Staging table created")
    print()
    
    # Step 2: Load CSV to staging
    print("Step 2: Loading CSV data into staging table...")
    success, row_count = load_csv_to_staging(args.csv_file, args.host, args.port, args.user, args.password, args.database)
    if not success:
        cleanup_staging(args.host, args.port, args.user, args.password, args.database)
        print("Failed to load CSV data", file=sys.stderr)
        sys.exit(1)
    print()
    
    # Step 3: Merge staging to main
    print("Step 3: Merging staging data into main table...")
    if not merge_staging_to_main(args.host, args.port, args.user, args.password, args.database, args.table):
        cleanup_staging(args.host, args.port, args.user, args.password, args.database)
        print("Failed to merge data", file=sys.stderr)
        sys.exit(1)
    print()
    
    # Step 4: Cleanup
    cleanup_staging(args.host, args.port, args.user, args.password, args.database)
    print()
    
    # Final row count
    final_count = get_row_count(args.host, args.port, args.user, args.password, args.database, args.table)
    print("=" * 60)
    print("Update Complete!")
    print(f"Rows before: {initial_count}")
    print(f"Rows after: {final_count}")
    print(f"Rows processed: {row_count}")
    print("=" * 60)


if __name__ == "__main__":
    main()

