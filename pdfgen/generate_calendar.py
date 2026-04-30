#!/usr/bin/env python3
import calendar
import json
import math
import os
import sys
from collections import defaultdict
from datetime import date, datetime, timedelta

import holidays
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4, landscape
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas


PAGE_WIDTH, PAGE_HEIGHT = landscape(A4)
MARGIN_LEFT = 8 * mm
MARGIN_RIGHT = 8 * mm
MARGIN_TOP = 8 * mm
MARGIN_BOTTOM = 8 * mm
HEADER_HEIGHT = 24 * mm
WEEKDAY_HEADER_HEIGHT = 9 * mm

COLORS = {
    "black": colors.HexColor("#111111"),
    "red": colors.HexColor("#b91c1c"),
    "blue": colors.HexColor("#1d4ed8"),
}

BAR_FILLS = {
    "black": colors.HexColor("#d1d5db"),
    "red": colors.HexColor("#fecaca"),
    "blue": colors.HexColor("#bfdbfe"),
}

MONTH_NAMES = [
    "",
    "January",
    "February",
    "March",
    "April",
    "May",
    "June",
    "July",
    "August",
    "September",
    "October",
    "November",
    "December",
]

WEEKDAY_LABELS = {
    0: ("月", "Mon"),
    1: ("火", "Tue"),
    2: ("水", "Wed"),
    3: ("木", "Thu"),
    4: ("金", "Fri"),
    5: ("土", "Sat"),
    6: ("日", "Sun"),
}


def parse_date(value):
    return datetime.strptime(value, "%Y-%m-%d").date()


def register_fonts():
    jp_body = os.getenv("PDF_FONT_JP_BODY", "/usr/share/fonts/truetype/bizud-gothic/BIZUDPGothic-Regular.ttf")
    jp_bold = os.getenv("PDF_FONT_JP_BOLD", "/usr/share/fonts/truetype/bizud-gothic/BIZUDGothic-Bold.ttf")
    latin_bold = os.getenv("PDF_FONT_LATIN_BOLD", "/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf")
    latin_semibold = os.getenv("PDF_FONT_LATIN_SEMIBOLD", "/usr/share/fonts/truetype/noto/NotoSans-SemiBold.ttf")
    latin_medium = os.getenv("PDF_FONT_LATIN_MEDIUM", "/usr/share/fonts/truetype/noto/NotoSans-Medium.ttf")
    required = [jp_body, jp_bold, latin_bold, latin_semibold, latin_medium]
    missing = [path for path in required if not os.path.exists(path)]
    if missing:
        raise FileNotFoundError(f"PDF font file not found: {', '.join(missing)}")
    pdfmetrics.registerFont(TTFont("JP-Body", jp_body))
    pdfmetrics.registerFont(TTFont("JP-Bold", jp_bold))
    pdfmetrics.registerFont(TTFont("Latin-Bold", latin_bold))
    pdfmetrics.registerFont(TTFont("Latin-SemiBold", latin_semibold))
    pdfmetrics.registerFont(TTFont("Latin-Medium", latin_medium))
    return {
        "jp_body": "JP-Body",
        "jp_bold": "JP-Bold",
        "latin_bold": "Latin-Bold",
        "latin_semibold": "Latin-SemiBold",
        "latin_medium": "Latin-Medium",
    }


FONTS = register_fonts()

TELEWORK_FONT_SIZE = 8.0
HOLIDAY_FONT_SIZE = 8.2
SCHEDULE_FONT_SIZE = 8.4
MULTI_DAY_FONT_SIZE = 7.8


def build_grid(year, month, week_starts_on):
    first = date(year, month, 1)
    last = date(year, month, calendar.monthrange(year, month)[1])
    week_start = 6 if week_starts_on == "sunday" else 0
    offset = (first.weekday() - week_start) % 7
    start = first - timedelta(days=offset)
    days = (last - start).days + 1
    weeks = math.ceil(days / 7)
    end = start + timedelta(days=weeks * 7 - 1)
    grid = []
    current = start
    while current <= end:
        row = []
        for _ in range(7):
            row.append(current)
            current += timedelta(days=1)
        grid.append(row)
    return grid


def holiday_map(year):
    data = holidays.country_holidays("JP", years=[year - 1, year, year + 1], language="ja")
    return {d.isoformat(): str(name) for d, name in data.items()}


def draw_header(c, year, month):
    center_x = PAGE_WIDTH / 2
    y = PAGE_HEIGHT - MARGIN_TOP - 13 * mm
    c.setFillColor(colors.HexColor("#111111"))
    c.setFont(FONTS["latin_bold"], 18)
    c.drawRightString(center_x - 18 * mm, y, str(year))
    c.setFont(FONTS["latin_bold"], 34)
    c.drawCentredString(center_x, y - 1.5 * mm, str(month))
    c.setFont(FONTS["latin_semibold"], 18)
    c.drawString(center_x + 18 * mm, y, MONTH_NAMES[month])


def day_tone(day, current_month, holidays_by_date):
    if day.month != current_month:
        return colors.HexColor("#9ca3af")
    if day.weekday() == 6 or day.isoformat() in holidays_by_date:
        return colors.HexColor("#9f2f2d")
    if day.weekday() == 5:
        return colors.HexColor("#365f91")
    return colors.HexColor("#111111")


def draw_weekday_headers(c, grid_x, grid_y_top, col_w, weekday_order):
    for col, weekday in enumerate(weekday_order):
        x = grid_x + col * col_w
        y = grid_y_top - WEEKDAY_HEADER_HEIGHT
        if weekday == 5:
            fill = colors.HexColor("#476b9e")
            text = colors.white
        elif weekday == 6:
            fill = colors.HexColor("#a25a4b")
            text = colors.white
        else:
            fill = colors.HexColor("#e5e7eb")
            text = colors.HexColor("#111111")
        c.setFillColor(fill)
        c.rect(x, y, col_w, WEEKDAY_HEADER_HEIGHT, stroke=0, fill=1)
        jp, en = WEEKDAY_LABELS[weekday]
        c.setFillColor(text)
        draw_centered_segments(
            c,
            x + col_w / 2,
            y + 2.6 * mm,
            [
                (FONTS["jp_bold"], 11, jp),
                (FONTS["latin_semibold"], 8.8, f" {en}"),
            ],
        )


def draw_centered_segments(c, center_x, y, segments):
    total_width = sum(c.stringWidth(text, font, size) for font, size, text in segments)
    x = center_x - total_width / 2
    for font, size, text in segments:
        c.setFont(font, size)
        c.drawString(x, y, text)
        x += c.stringWidth(text, font, size)


def wrap_text(c, text, font, size, max_width, max_lines):
    result = []
    for raw in str(text).splitlines():
        line = ""
        for ch in raw:
            test = line + ch
            if c.stringWidth(test, font, size) <= max_width:
                line = test
            else:
                if line:
                    result.append(line)
                line = ch
                if len(result) >= max_lines:
                    return result
        if line:
            result.append(line)
        if len(result) >= max_lines:
            return result
    return result[:max_lines]


def is_latin_numeric_char(ch):
    return 0x20 <= ord(ch) <= 0x7E


def mixed_string_width(c, text, size, jp_font, latin_font):
    total = 0
    for ch in text:
        total += c.stringWidth(ch, latin_font if is_latin_numeric_char(ch) else jp_font, size)
    return total


def wrap_mixed_text(c, text, size, max_width, max_lines, jp_font, latin_font):
    result = []
    for raw in str(text).splitlines():
        line = ""
        for ch in raw:
            test = line + ch
            if mixed_string_width(c, test, size, jp_font, latin_font) <= max_width:
                line = test
            else:
                if line:
                    result.append(line)
                line = ch
                if len(result) >= max_lines:
                    return result
        if line:
            result.append(line)
        if len(result) >= max_lines:
            return result
    return result[:max_lines]


def draw_mixed_string(c, x, y, text, size, jp_font, latin_font):
    current_x = x
    current_font = None
    current_text = ""
    for ch in str(text):
        font = latin_font if is_latin_numeric_char(ch) else jp_font
        if current_font is None:
            current_font = font
        if font != current_font:
            c.setFont(current_font, size)
            c.drawString(current_x, y, current_text)
            current_x += c.stringWidth(current_text, current_font, size)
            current_font = font
            current_text = ch
        else:
            current_text += ch
    if current_text:
        c.setFont(current_font, size)
        c.drawString(current_x, y, current_text)


def draw_mixed_centered(c, center_x, y, text, size, jp_font, latin_font):
    width = mixed_string_width(c, text, size, jp_font, latin_font)
    draw_mixed_string(c, center_x - width / 2, y, text, size, jp_font, latin_font)


def multi_lane_counts_by_cell(req, grid):
    lanes_by_row = defaultdict(int)
    lanes_by_cell = defaultdict(int)
    sorted_items = sorted(req.get("multiDayItems") or [], key=lambda item: (item["startDate"], item["endDate"], item.get("id", "")))
    for item in sorted_items:
        for segment in split_multi_day_item(item, grid):
            row = segment["row"]
            lane = lanes_by_row[row]
            lanes_by_row[row] += 1
            if lane > 3:
                continue
            for col in range(segment["start_col"], segment["end_col"] + 1):
                lanes_by_cell[(row, col)] = max(lanes_by_cell[(row, col)], lane + 1)
    return lanes_by_cell


def draw_day_content(c, req, grid, layout, holidays_by_date, lanes_by_cell):
    telework = req.get("telework") or {}
    schedule_by_date = defaultdict(list)
    for item in req.get("scheduleItems") or []:
        schedule_by_date[item["date"]].append(item)

    for row_index, row in enumerate(grid):
        for col_index, day in enumerate(row):
            x = layout["x"] + col_index * layout["col_w"]
            y_top = layout["body_top"] - row_index * layout["row_h"]
            y_bottom = y_top - layout["row_h"]
            key = day.isoformat()

            if day.month != req["month"]:
                c.setFillColor(colors.HexColor("#f3f4f6"))
                c.rect(x, y_bottom, layout["col_w"], layout["row_h"], stroke=0, fill=1)
            elif day.weekday() == 6 or key in holidays_by_date:
                c.setFillColor(colors.HexColor("#fff8f8"))
                c.rect(x, y_bottom, layout["col_w"], layout["row_h"], stroke=0, fill=1)
            elif day.weekday() == 5:
                c.setFillColor(colors.HexColor("#f5f9ff"))
                c.rect(x, y_bottom, layout["col_w"], layout["row_h"], stroke=0, fill=1)

            c.setFillColor(day_tone(day, req["month"], holidays_by_date))
            c.setFont(FONTS["latin_bold"], 18)
            c.drawString(x + 2.2 * mm, y_top - 7.6 * mm, str(day.day))

            label_x = x + 13 * mm
            label_y = y_top - 5.1 * mm
            status = telework.get(key, {})
            labels = []
            if status.get("papa"):
                labels.append("パパテレワーク")
            if status.get("mama"):
                labels.append("ママテレワーク")
            c.setFont(FONTS["jp_bold"], TELEWORK_FONT_SIZE)
            telework_width = 0
            for idx, label in enumerate(labels):
                c.setFillColor(colors.HexColor("#374151"))
                c.drawString(label_x, label_y - idx * 3.6 * mm, label)
                telework_width = max(telework_width, c.stringWidth(label, FONTS["jp_bold"], TELEWORK_FONT_SIZE))

            if key in holidays_by_date:
                holiday_x = label_x + telework_width + (2.0 * mm if telework_width > 0 else 0)
                max_holiday_width = x + layout["col_w"] - 1.8 * mm - holiday_x
                holiday_name = holidays_by_date[key]
                while holiday_name and c.stringWidth(holiday_name, FONTS["jp_body"], HOLIDAY_FONT_SIZE) > max_holiday_width:
                    holiday_name = holiday_name[:-1]
                c.setFillColor(day_tone(day, req["month"], holidays_by_date))
                c.setFont(FONTS["jp_body"], HOLIDAY_FONT_SIZE)
                c.drawString(holiday_x, label_y, holiday_name)

            reserved_multi_h = lanes_by_cell.get((row_index, col_index), 0) * 5.8 * mm
            item_y = y_top - 12 * mm - reserved_multi_h
            c.setFont(FONTS["jp_body"], SCHEDULE_FONT_SIZE)
            for item in schedule_by_date.get(key, [])[:5]:
                c.setFillColor(COLORS.get(item.get("color", "black"), COLORS["black"]))
                for line in wrap_mixed_text(
                    c,
                    item.get("text", ""),
                    SCHEDULE_FONT_SIZE,
                    layout["col_w"] - 4.5 * mm,
                    2,
                    FONTS["jp_body"],
                    FONTS["latin_medium"],
                ):
                    if item_y < y_bottom + 3.5 * mm:
                        break
                    draw_mixed_string(c, x + 2.2 * mm, item_y, line, SCHEDULE_FONT_SIZE, FONTS["jp_body"], FONTS["latin_medium"])
                    item_y -= 4.4 * mm


def split_multi_day_item(item, grid):
    start = parse_date(item["startDate"])
    end = parse_date(item["endDate"])
    segments = []
    for row_index, row in enumerate(grid):
        row_start = row[0]
        row_end = row[-1]
        seg_start = max(start, row_start)
        seg_end = min(end, row_end)
        if seg_start <= seg_end:
            segments.append(
                {
                    "row": row_index,
                    "start_col": (seg_start - row_start).days,
                    "end_col": (seg_end - row_start).days,
                    "is_first": seg_start == start,
                    "is_last": seg_end == end,
                }
            )
    return segments


def draw_multi_day_items(c, req, grid, layout):
    lanes_by_row = defaultdict(int)
    sorted_items = sorted(req.get("multiDayItems") or [], key=lambda item: (item["startDate"], item["endDate"], item.get("id", "")))
    for item in sorted_items:
        for segment in split_multi_day_item(item, grid):
            row = segment["row"]
            lane = lanes_by_row[row]
            lanes_by_row[row] += 1
            if lane > 3:
                continue

            x1 = layout["x"] + segment["start_col"] * layout["col_w"] + 1.8 * mm
            x2 = layout["x"] + (segment["end_col"] + 1) * layout["col_w"] - 1.8 * mm
            y_top = layout["body_top"] - row * layout["row_h"]
            bar_h = 4.7 * mm
            y = y_top - (11.4 + lane * 5.8) * mm
            row_bottom = y_top - layout["row_h"] + 3 * mm
            if y < row_bottom:
                y = row_bottom

            item_color = item.get("color", "black")
            stroke = COLORS.get(item_color, COLORS["black"])
            fill = BAR_FILLS.get(item_color, BAR_FILLS["black"])
            c.setStrokeColor(stroke)
            c.setFillColor(fill)
            c.setLineWidth(0.65)
            c.roundRect(x1, y - bar_h / 2, x2 - x1, bar_h, 1.1 * mm, stroke=1, fill=1)
            text = item.get("text", "")
            if text:
                max_width = max(1, x2 - x1 - 3 * mm)
                display_text = text
                while display_text and mixed_string_width(c, display_text, MULTI_DAY_FONT_SIZE, FONTS["jp_body"], FONTS["latin_medium"]) > max_width:
                    display_text = display_text[:-1]
                c.setFillColor(colors.HexColor("#111827"))
                draw_mixed_centered(c, (x1 + x2) / 2, y - 1.3 * mm, display_text, MULTI_DAY_FONT_SIZE, FONTS["jp_body"], FONTS["latin_medium"])


def draw_grid_lines(c, layout, weeks):
    c.setStrokeColor(colors.HexColor("#1f2937"))
    c.setLineWidth(0.75)
    total_h = WEEKDAY_HEADER_HEIGHT + weeks * layout["row_h"]
    c.rect(layout["x"], layout["top"] - total_h, layout["width"], total_h, stroke=1, fill=0)
    for col in range(1, 7):
        x = layout["x"] + col * layout["col_w"]
        c.line(x, layout["top"], x, layout["top"] - total_h)
    c.line(layout["x"], layout["body_top"], layout["x"] + layout["width"], layout["body_top"])
    for row in range(1, weeks):
        y = layout["body_top"] - row * layout["row_h"]
        c.line(layout["x"], y, layout["x"] + layout["width"], y)


def generate(req):
    year = int(req["year"])
    month = int(req["month"])
    week_starts_on = req.get("weekStartsOn") or "monday"
    grid = build_grid(year, month, week_starts_on)
    weeks = len(grid)
    holidays_by_date = holiday_map(year)

    calendar_width = PAGE_WIDTH - MARGIN_LEFT - MARGIN_RIGHT
    calendar_height = PAGE_HEIGHT - MARGIN_TOP - MARGIN_BOTTOM - HEADER_HEIGHT
    col_w = calendar_width / 7
    row_h = (calendar_height - WEEKDAY_HEADER_HEIGHT) / weeks
    top = PAGE_HEIGHT - MARGIN_TOP - HEADER_HEIGHT
    layout = {
        "x": MARGIN_LEFT,
        "top": top,
        "body_top": top - WEEKDAY_HEADER_HEIGHT,
        "width": calendar_width,
        "col_w": col_w,
        "row_h": row_h,
    }

    c = canvas.Canvas(sys.stdout.buffer, pagesize=landscape(A4), pageCompression=1)
    c.setTitle(f"{year}-{month:02d} family calendar")
    draw_header(c, year, month)
    weekday_order = [6, 0, 1, 2, 3, 4, 5] if week_starts_on == "sunday" else [0, 1, 2, 3, 4, 5, 6]
    draw_weekday_headers(c, layout["x"], layout["top"], col_w, weekday_order)
    lanes_by_cell = multi_lane_counts_by_cell(req, grid)
    draw_day_content(c, req, grid, layout, holidays_by_date, lanes_by_cell)
    draw_grid_lines(c, layout, weeks)
    draw_multi_day_items(c, req, grid, layout)
    c.showPage()
    c.save()


def main():
    try:
        req = json.load(sys.stdin)
        generate(req)
    except Exception as exc:
        print(f"PDF generation error: {exc}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
