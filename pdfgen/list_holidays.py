#!/usr/bin/env python3
import json
import sys

import holidays


def main():
    if len(sys.argv) != 2:
        print("usage: list_holidays.py YEAR", file=sys.stderr)
        sys.exit(2)
    year = int(sys.argv[1])
    data = holidays.country_holidays("JP", years=[year - 1, year, year + 1], language="ja")
    payload = [{"date": day.isoformat(), "name": str(name)} for day, name in sorted(data.items())]
    json.dump(payload, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
