<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { Button, Checkbox, Input, Modal } from 'flowbite-svelte';
  import { CalendarDays, ChevronDown, Download, Eye, GripVertical, Plus, Trash2 } from 'lucide-svelte';

  type WeekStartsOn = 'monday' | 'sunday';
  type ItemColor = 'black' | 'red' | 'blue';
  type TeleworkStatus = { papa: boolean; mama: boolean };
  type ScheduleItem = { id: string; date: string; text: string; color: ItemColor };
  type MultiDayScheduleItem = {
    id: string;
    startDate: string;
    endDate: string;
    text: string;
    color: ItemColor;
    arrow: boolean;
  };
  type CalendarData = {
    year: number;
    month: number;
    weekStartsOn: WeekStartsOn;
    telework: Record<string, TeleworkStatus>;
    scheduleItems: ScheduleItem[];
    multiDayItems: MultiDayScheduleItem[];
  };
  type Segment = {
    id: string;
    row: number;
    startCol: number;
    endCol: number;
    lane: number;
    text: string;
    color: ItemColor;
    arrow: boolean;
    isLast: boolean;
  };

  const legacyStorageKey = 'kazokucal.calendar.v1';
  const monthNames = [
    '',
    'January',
    'February',
    'March',
    'April',
    'May',
    'June',
    'July',
    'August',
    'September',
    'October',
    'November',
    'December'
  ];
  const weekdays = {
    0: { jp: '日', en: 'Sun' },
    1: { jp: '月', en: 'Mon' },
    2: { jp: '火', en: 'Tue' },
    3: { jp: '水', en: 'Wed' },
    4: { jp: '木', en: 'Thu' },
    5: { jp: '金', en: 'Fri' },
    6: { jp: '土', en: 'Sat' }
  } as const;
  const colorOptions: ItemColor[] = ['black', 'red', 'blue'];
  const colorLabels: Record<ItemColor, string> = { black: '黒', red: '赤', blue: '青' };

  let data: CalendarData = defaultData();
  let ready = false;
  let loadBusy = false;
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  let holidays: Record<string, string> = {};
  let loadedHolidayYear: number | null = null;
  let selectedDayKey = '';
  let dayModalOpen = false;
  let dayEditorMode: 'schedule' | 'multi' = 'schedule';
  let multiDayModalOpen = false;
  let selectedMultiDayId = '';
  let newScheduleText = '';
  let newScheduleColor: ItemColor = 'black';
  let newMultiDay: MultiDayScheduleItem = blankMultiDay();
  let dayModalMessage = '';
  let dataMenuOpen = false;
  let replaceOnImport = false;
  let importBusy = false;
  let importResultOpen = false;
  let importResultTitle = '';
  let importResultMessage = '';
  let statusMessage = '';
  let downloadBusy = false;
  let previewBusy = false;
  let draggingScheduleId = '';
  let dragOverScheduleId = '';

  $: grid = buildGrid(data.year, data.month, data.weekStartsOn);
  $: weekdayOrder = data.weekStartsOn === 'sunday' ? [0, 1, 2, 3, 4, 5, 6] : [1, 2, 3, 4, 5, 6, 0];
  $: selectedTelework = data.telework[selectedDayKey] ?? { papa: false, mama: false };
  $: selectedItems = data.scheduleItems.filter((item) => item.date === selectedDayKey);
  $: selectedMultiDayItem = data.multiDayItems.find((item) => item.id === selectedMultiDayId);
  $: multiDaySegments = buildMultiDaySegments(data.multiDayItems, grid);
  $: multiDaySegmentsByRow = groupSegmentsByRow(multiDaySegments, grid.length);
  $: pdfBusy = downloadBusy || previewBusy;
  $: if (ready && data.year !== loadedHolidayYear) {
    void loadHolidays(data.year);
  }

  onMount(() => {
    localStorage.removeItem(legacyStorageKey);
    const initial = defaultData();
    void loadCalendar(initial.year, initial.month, true);
  });

  onDestroy(() => {
    removeScheduleDragListeners();
  });

  function apiPath(path: string): string {
    return `api/${path.replace(/^\/+/, '')}`;
  }

  function defaultData(): CalendarData {
    const now = new Date();
    return {
      year: now.getFullYear(),
      month: now.getMonth() + 1,
      weekStartsOn: 'monday',
      telework: {},
      scheduleItems: [],
      multiDayItems: []
    };
  }

  function normalizeData(value: Partial<CalendarData>): CalendarData {
    const current = defaultData();
    const year = Number(value.year ?? current.year);
    const month = Number(value.month ?? current.month);
    return {
      year: Math.min(2100, Math.max(1900, Number.isFinite(year) ? year : current.year)),
      month: Math.min(12, Math.max(1, Number.isFinite(month) ? month : current.month)),
      weekStartsOn: value.weekStartsOn === 'sunday' ? 'sunday' : 'monday',
      telework: value.telework ?? {},
      scheduleItems: Array.isArray(value.scheduleItems) ? value.scheduleItems : [],
      multiDayItems: Array.isArray(value.multiDayItems) ? value.multiDayItems : []
    };
  }

  function mergeById<T extends { id: string }>(base: T[], incoming: T[]): T[] {
    const merged = [...base];
    const indexes = new Map(merged.map((item, index) => [item.id, index]));
    for (const item of incoming) {
      const index = indexes.get(item.id);
      if (index === undefined) {
        indexes.set(item.id, merged.length);
        merged.push(item);
      } else {
        merged[index] = item;
      }
    }
    return merged;
  }

  function mergeCalendarData(base: CalendarData, incoming: CalendarData): CalendarData {
    return {
      ...base,
      year: incoming.year,
      month: incoming.month,
      weekStartsOn: incoming.weekStartsOn,
      telework: { ...base.telework, ...incoming.telework },
      scheduleItems: mergeById(base.scheduleItems, incoming.scheduleItems),
      multiDayItems: mergeById(base.multiDayItems, incoming.multiDayItems)
    };
  }

  async function fetchCalendar(year: number, month: number): Promise<CalendarData> {
    const response = await fetch(apiPath(`calendar?year=${year}&month=${month}`));
    if (!response.ok) throw new Error(await response.text());
    return normalizeData((await response.json()) as Partial<CalendarData>);
  }

  function blankMultiDay(): MultiDayScheduleItem {
    const start = `${data.year}-${pad(data.month)}-01`;
    return { id: createId(), startDate: start, endDate: start, text: '', color: 'black', arrow: true };
  }

  function blankMultiDayForDate(date: string): MultiDayScheduleItem {
    const start = normalizeDateKey(date) || `${data.year}-${pad(data.month)}-01`;
    return { id: createId(), startDate: start, endDate: start, text: '', color: 'black', arrow: true };
  }

  function commitData(next: CalendarData) {
    data = next;
    scheduleSave();
  }

  function createId(): string {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
      const bytes = new Uint8Array(16);
      crypto.getRandomValues(bytes);
      bytes[6] = (bytes[6] & 0x0f) | 0x40;
      bytes[8] = (bytes[8] & 0x3f) | 0x80;
      const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
      return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
    }
    return `id-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
  }

  function pad(value: number): string {
    return String(value).padStart(2, '0');
  }

  function dateKey(day: Date): string {
    return `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}`;
  }

  function normalizeDateKey(value: string): string {
    const match = String(value).trim().match(/^(\d{4})[-/](\d{1,2})[-/](\d{1,2})$/);
    if (!match) return '';
    return `${match[1]}-${pad(Number(match[2]))}-${pad(Number(match[3]))}`;
  }

  function parseKey(key: string): Date {
    const [year, month, day] = normalizeDateKey(key).split('-').map(Number);
    return new Date(year, month - 1, day);
  }

  function addDays(day: Date, amount: number): Date {
    return new Date(day.getFullYear(), day.getMonth(), day.getDate() + amount);
  }

  function buildGrid(year: number, month: number, weekStartsOn: WeekStartsOn): Date[][] {
    const first = new Date(year, month - 1, 1);
    const last = new Date(year, month, 0);
    const startDay = weekStartsOn === 'sunday' ? 0 : 1;
    const offset = (first.getDay() - startDay + 7) % 7;
    const start = addDays(first, -offset);
    const dayCount = Math.round((last.getTime() - start.getTime()) / 86400000) + 1;
    const weeks = Math.ceil(dayCount / 7);
    return Array.from({ length: weeks }, (_, row) =>
      Array.from({ length: 7 }, (_unused, col) => addDays(start, row * 7 + col))
    );
  }

  async function loadHolidays(year: number) {
    loadedHolidayYear = year;
    try {
      const response = await fetch(apiPath(`holidays?year=${year}`));
      if (!response.ok) throw new Error(await response.text());
      const payload = (await response.json()) as { holidays: Array<{ date: string; name: string }> };
      holidays = Object.fromEntries(payload.holidays.map((holiday) => [holiday.date, holiday.name]));
    } catch {
      holidays = {};
    }
  }

  function updateYear(value: string) {
    const year = Math.min(2100, Math.max(1900, Number(value)));
    void changeCalendarPeriod(year, data.month);
  }

  function updateMonth(value: string) {
    const month = Math.min(12, Math.max(1, Number(value)));
    void changeCalendarPeriod(data.year, month);
  }

  function updateWeekStartsOn(value: string) {
    commitData({ ...data, weekStartsOn: value === 'sunday' ? 'sunday' : 'monday' });
  }

  function openDay(day: Date) {
    selectedDayKey = dateKey(day);
    dayEditorMode = 'schedule';
    newScheduleText = '';
    newScheduleColor = 'black';
    newMultiDay = blankMultiDayForDate(selectedDayKey);
    dayModalMessage = '';
    dayModalOpen = true;
  }

  function switchDayEditorMode(mode: 'schedule' | 'multi') {
    dayEditorMode = mode;
    dayModalMessage = '';
    if (mode === 'multi') {
      newMultiDay = { ...newMultiDay, startDate: selectedDayKey, endDate: selectedDayKey };
    }
  }

  function handleDayKeydown(event: KeyboardEvent, day: Date) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      openDay(day);
    }
  }

  function setTelework(person: keyof TeleworkStatus, checked: boolean) {
    const current = data.telework[selectedDayKey] ?? { papa: false, mama: false };
    commitData({
      ...data,
      telework: {
        ...data.telework,
        [selectedDayKey]: { ...current, [person]: checked }
      }
    });
  }

  function addSchedule() {
    const text = newScheduleText.trim();
    if (!text) return;
    commitData({
      ...data,
      scheduleItems: [...data.scheduleItems, { id: createId(), date: selectedDayKey, text, color: newScheduleColor }]
    });
    newScheduleText = '';
  }

  function updateSchedule(id: string, patch: Partial<ScheduleItem>) {
    commitData({
      ...data,
      scheduleItems: data.scheduleItems.map((item) => (item.id === id ? { ...item, ...patch } : item))
    });
  }

  function deleteSchedule(id: string) {
    commitData({ ...data, scheduleItems: data.scheduleItems.filter((item) => item.id !== id) });
  }

  function reorderSelectedSchedule(sourceId: string, targetId: string) {
    if (sourceId === targetId) return;
    const dayItems = data.scheduleItems.filter((item) => item.date === selectedDayKey);
    const sourceIndex = dayItems.findIndex((item) => item.id === sourceId);
    const targetIndex = dayItems.findIndex((item) => item.id === targetId);
    if (sourceIndex < 0 || targetIndex < 0) return;

    const reordered = [...dayItems];
    const [moved] = reordered.splice(sourceIndex, 1);
    reordered.splice(targetIndex, 0, moved);
    let nextDayItemIndex = 0;
    commitData({
      ...data,
      scheduleItems: data.scheduleItems.map((item) => (item.date === selectedDayKey ? reordered[nextDayItemIndex++] : item))
    });
  }

  function startScheduleDrag(event: PointerEvent, id: string) {
    if (selectedItems.length < 2) return;
    draggingScheduleId = id;
    dragOverScheduleId = id;
    window.addEventListener('pointermove', handleSchedulePointerMove, { passive: false });
    window.addEventListener('pointerup', finishScheduleDrag);
    window.addEventListener('pointercancel', finishScheduleDrag);
    event.preventDefault();
  }

  function handleSchedulePointerMove(event: PointerEvent) {
    if (!draggingScheduleId) return;
    event.preventDefault();
    const target = document.elementFromPoint(event.clientX, event.clientY);
    const row = target?.closest<HTMLElement>('[data-schedule-id]');
    const targetId = row?.dataset.scheduleId;
    if (!targetId || targetId === dragOverScheduleId || !selectedItems.some((item) => item.id === targetId)) return;
    dragOverScheduleId = targetId;
    reorderSelectedSchedule(draggingScheduleId, targetId);
  }

  function finishScheduleDrag() {
    removeScheduleDragListeners();
    draggingScheduleId = '';
    dragOverScheduleId = '';
  }

  function removeScheduleDragListeners() {
    window.removeEventListener('pointermove', handleSchedulePointerMove);
    window.removeEventListener('pointerup', finishScheduleDrag);
    window.removeEventListener('pointercancel', finishScheduleDrag);
  }

  function handleScheduleHandleKeydown(event: KeyboardEvent, id: string) {
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
    const index = selectedItems.findIndex((item) => item.id === id);
    const targetIndex = event.key === 'ArrowUp' ? index - 1 : index + 1;
    const target = selectedItems[targetIndex];
    if (!target) return;
    event.preventDefault();
    reorderSelectedSchedule(id, target.id);
  }

  function addMultiDay() {
    const startDate = normalizeDateKey(newMultiDay.startDate);
    const endDate = normalizeDateKey(newMultiDay.endDate);
    if (!startDate || !endDate || startDate > endDate) {
      dayModalMessage = '期間予定の開始日と終了日を確認してください。';
      return;
    }
    commitData({ ...data, multiDayItems: [...data.multiDayItems, { ...newMultiDay, id: createId(), startDate, endDate }] });
    newMultiDay = blankMultiDayForDate(selectedDayKey);
    dayModalMessage = '';
    dayEditorMode = 'schedule';
  }

  function updateMultiDay(id: string, patch: Partial<MultiDayScheduleItem>) {
    if (patch.startDate) patch.startDate = normalizeDateKey(patch.startDate) || patch.startDate;
    if (patch.endDate) patch.endDate = normalizeDateKey(patch.endDate) || patch.endDate;
    commitData({
      ...data,
      multiDayItems: data.multiDayItems.map((item) => {
        if (item.id !== id) return item;
        const updated = { ...item, ...patch };
        const start = normalizeDateKey(updated.startDate);
        const end = normalizeDateKey(updated.endDate);
        if (start && end && start > end) {
          if (patch.startDate) updated.endDate = start;
          if (patch.endDate) updated.startDate = end;
        }
        return updated;
      })
    });
  }

  function deleteMultiDay(id: string) {
    commitData({ ...data, multiDayItems: data.multiDayItems.filter((item) => item.id !== id) });
    if (selectedMultiDayId === id) {
      selectedMultiDayId = '';
      multiDayModalOpen = false;
    }
  }

  function openMultiDay(id: string) {
    selectedMultiDayId = id;
    multiDayModalOpen = true;
  }

  function scheduleForDate(key: string): ScheduleItem[] {
    return data.scheduleItems.filter((item) => item.date === key);
  }

  function teleworkForDate(key: string): TeleworkStatus {
    return data.telework[key] ?? { papa: false, mama: false };
  }

  function buildMultiDaySegments(items: MultiDayScheduleItem[], calendarGrid: Date[][]): Segment[] {
    const segments: Segment[] = [];
    const lanes = new Map<number, number>();
    const sorted = [...items].sort((a, b) =>
      `${normalizeDateKey(a.startDate)}-${normalizeDateKey(a.endDate)}`.localeCompare(`${normalizeDateKey(b.startDate)}-${normalizeDateKey(b.endDate)}`)
    );
    for (const item of sorted) {
      const itemStartDate = normalizeDateKey(item.startDate);
      const itemEndDate = normalizeDateKey(item.endDate);
      if (!itemStartDate || !itemEndDate) continue;
      const start = parseKey(item.startDate);
      const end = parseKey(item.endDate);
      for (let row = 0; row < calendarGrid.length; row += 1) {
        const rowStart = calendarGrid[row][0];
        const rowEnd = calendarGrid[row][6];
        const segmentStart = start > rowStart ? start : rowStart;
        const segmentEnd = end < rowEnd ? end : rowEnd;
        if (segmentStart <= segmentEnd) {
          const lane = lanes.get(row) ?? 0;
          lanes.set(row, lane + 1);
          if (lane < 4) {
            segments.push({
              id: item.id,
              row,
              startCol: Math.round((segmentStart.getTime() - rowStart.getTime()) / 86400000),
              endCol: Math.round((segmentEnd.getTime() - rowStart.getTime()) / 86400000),
              lane,
              text: item.text,
              color: item.color,
              arrow: item.arrow,
              isLast: dateKey(segmentEnd) === itemEndDate
            });
          }
        }
      }
    }
    return segments;
  }

  function groupSegmentsByRow(segments: Segment[], rowCount: number): Segment[][] {
    const grouped = Array.from({ length: rowCount }, () => [] as Segment[]);
    for (const segment of segments) {
      grouped[segment.row]?.push(segment);
    }
    return grouped;
  }

  function multiLaneCountForCell(segments: Segment[] | undefined, col: number): number {
    if (!segments?.length) return 0;
    const covering = segments.filter((segment) => segment.startCol <= col && col <= segment.endCol);
    if (!covering.length) return 0;
    return Math.max(...covering.map((segment) => segment.lane)) + 1;
  }

  function colorClass(color: ItemColor): string {
    return `item-${color}`;
  }

  function colorHex(color: ItemColor): string {
    if (color === 'red') return '#b91c1c';
    if (color === 'blue') return '#1d4ed8';
    return '#111111';
  }

  function dayClass(day: Date): string {
    const key = dateKey(day);
    const classes = ['day-cell'];
    if (day.getMonth() + 1 !== data.month) classes.push('outside');
    if (day.getDay() === 0 || holidays[key]) classes.push('sun-holiday');
    if (day.getDay() === 6) classes.push('sat');
    return classes.join(' ');
  }

  function weekdayText(weekday: number): string {
    const label = weekdays[weekday as keyof typeof weekdays];
    return `${label.jp} ${label.en}`;
  }

  function coerceColor(value: string): ItemColor {
    return value === 'red' || value === 'blue' ? value : 'black';
  }

  async function generatePdfBlob(): Promise<Blob> {
    await saveCalendarNow();
    const response = await fetch(apiPath('pdf'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text);
    }
    return response.blob();
  }

  async function downloadPdf() {
    downloadBusy = true;
    statusMessage = '';
    try {
      const blob = await generatePdfBlob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `calendar-${data.year}-${pad(data.month)}.pdf`;
      link.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      statusMessage = error instanceof Error ? error.message : 'PDF の生成に失敗しました。';
    } finally {
      downloadBusy = false;
    }
  }

  async function previewPdf() {
    previewBusy = true;
    statusMessage = '';
    const previewWindow = window.open('', '_blank');
    if (!previewWindow) {
      previewBusy = false;
      statusMessage = 'PDF 表示用のタブを開けませんでした。ブラウザのポップアップ設定を確認してください。';
      return;
    }
    previewWindow.document.title = 'PDF 生成中';
    previewWindow.document.body.textContent = 'PDF を生成しています...';
    try {
      const blob = await generatePdfBlob();
      const url = URL.createObjectURL(blob);
      previewWindow.location.href = url;
      setTimeout(() => URL.revokeObjectURL(url), 60000);
    } catch (error) {
      previewWindow.close();
      statusMessage = error instanceof Error ? error.message : 'PDF の生成に失敗しました。';
    } finally {
      previewBusy = false;
    }
  }

  function exportJson() {
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `kazokucal-${data.year}-${pad(data.month)}.json`;
    link.click();
    URL.revokeObjectURL(url);
  }

  async function importJson(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    statusMessage = '';
    importBusy = true;
    importResultOpen = false;
    try {
      const imported = normalizeData(JSON.parse(await file.text()) as Partial<CalendarData>);
      if (replaceOnImport) {
        commitData(imported);
        selectedDayKey = `${imported.year}-${pad(imported.month)}-01`;
        newMultiDay = blankMultiDayForDate(selectedDayKey);
      } else if (imported.year === data.year && imported.month === data.month) {
        commitData(mergeCalendarData(data, imported));
        selectedDayKey = `${imported.year}-${pad(imported.month)}-01`;
        newMultiDay = blankMultiDayForDate(selectedDayKey);
      } else {
        await saveCalendarNow();
        const existing = await fetchCalendar(imported.year, imported.month);
        commitData(mergeCalendarData(existing, imported));
        selectedDayKey = `${imported.year}-${pad(imported.month)}-01`;
        newMultiDay = blankMultiDayForDate(selectedDayKey);
      }
      await saveCalendarNow();
      dataMenuOpen = false;
      importResultTitle = '読み込み完了';
      importResultMessage = `${imported.year}年${imported.month}月のデータを${replaceOnImport ? '入れ替え' : '読み込み'}ました。`;
    } catch (error) {
      importResultTitle = '読み込み失敗';
      importResultMessage = error instanceof Error ? error.message : 'JSON の読み込みに失敗しました。';
    } finally {
      importBusy = false;
      importResultOpen = true;
      input.value = '';
    }
  }

  async function changeCalendarPeriod(year: number, month: number) {
    try {
      await saveCalendarNow();
      await loadCalendar(year, month, false);
    } catch (error) {
      statusMessage = error instanceof Error ? error.message : 'カレンダーの保存または読み込みに失敗しました。';
    }
  }

  async function loadCalendar(year: number, month: number, initial: boolean) {
    loadBusy = true;
    statusMessage = '';
    try {
      const response = await fetch(apiPath(`calendar?year=${year}&month=${month}`));
      if (!response.ok) throw new Error(await response.text());
      data = normalizeData((await response.json()) as Partial<CalendarData>);
      selectedDayKey = `${data.year}-${pad(data.month)}-01`;
      newMultiDay = blankMultiDay();
      if (initial) ready = true;
    } catch (error) {
      data = normalizeData({ year, month });
      selectedDayKey = `${data.year}-${pad(data.month)}-01`;
      newMultiDay = blankMultiDay();
      statusMessage = error instanceof Error ? error.message : 'カレンダーの読み込みに失敗しました。';
      if (initial) ready = true;
    } finally {
      loadBusy = false;
    }
  }

  function scheduleSave() {
    if (!ready || loadBusy) return;
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      void saveCalendarNow().catch((error) => {
        statusMessage = error instanceof Error ? error.message : 'カレンダーの保存に失敗しました。';
      });
    }, 600);
  }

  async function saveCalendarNow() {
    if (!ready || loadBusy) return;
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = undefined;
    }
    const response = await fetch(apiPath('calendar'), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
  }
</script>

<main class="app-shell">
  <section class="toolbar">
    <div class="title-row">
      <CalendarDays size={30} />
      <div>
        <h1>KazokuCal</h1>
        <p>{data.year}年 {data.month}月 / {monthNames[data.month]}</p>
      </div>
    </div>

    <div class="controls">
      <label>
        年
        <input class="select-like year-input" type="number" min="1900" max="2100" value={data.year} on:change={(e) => updateYear(e.currentTarget.value)} />
      </label>
      <label>
        月
        <select class="select-like" value={data.month} on:change={(e) => updateMonth(e.currentTarget.value)}>
          {#each Array.from({ length: 12 }, (_, i) => i + 1) as month}
            <option value={month}>{month}月</option>
          {/each}
        </select>
      </label>
      <label>
        週の始まり
        <select class="select-like" value={data.weekStartsOn} on:change={(e) => updateWeekStartsOn(e.currentTarget.value)}>
          <option value="monday">月曜日</option>
          <option value="sunday">日曜日</option>
        </select>
      </label>
      <Button color="light" on:click={previewPdf} disabled={pdfBusy}>
        <Eye size={16} class="mr-2" />
        {previewBusy ? '生成中' : 'PDF 表示'}
      </Button>
      <Button color="blue" on:click={downloadPdf} disabled={pdfBusy}>
        <Download size={16} class="mr-2" />
        {downloadBusy ? '生成中' : 'PDF ダウンロード'}
      </Button>
      <div class="toolbar-data">
        <Button color="light" on:click={() => (dataMenuOpen = !dataMenuOpen)} aria-expanded={dataMenuOpen}>
          データ
          <ChevronDown size={16} class={`ml-2 data-chevron ${dataMenuOpen ? 'open' : ''}`} />
        </Button>
        {#if dataMenuOpen}
          <div class="toolbar-data-menu">
            <Button color="light" on:click={exportJson}>JSON 書き出し</Button>
            <label class="file-button">
              JSON 読み込み
              <input type="file" accept="application/json" on:change={importJson} />
            </label>
            <Checkbox bind:checked={replaceOnImport}>既存予定を消して入れ替え</Checkbox>
          </div>
        {/if}
      </div>
    </div>
  </section>

  {#if statusMessage}
    <p class="status">{statusMessage}</p>
  {/if}

  <section class="calendar-preview" aria-label="カレンダープレビュー">
    <div class="calendar-title">
      <span>{data.year}</span>
      <strong>{data.month}</strong>
      <span>{monthNames[data.month]}</span>
    </div>
    <div class="weekday-grid">
      {#each weekdayOrder as weekday}
        <div class:weekday-sat={weekday === 6} class:weekday-sun={weekday === 0}>
          {weekdayText(weekday)}
        </div>
      {/each}
    </div>
    <div class="week-stack">
      {#each grid as week, rowIndex}
        {@const rowSegments = multiDaySegmentsByRow[rowIndex] ?? []}
        <div class="week-row">
          {#each week as day, colIndex}
            {@const key = dateKey(day)}
            {@const status = teleworkForDate(key)}
            {@const dayLaneCount = multiLaneCountForCell(rowSegments, colIndex)}
            <div
              class={dayClass(day)}
              class:has-multi-day={dayLaneCount > 0}
              style={`--day-multi-lanes: ${dayLaneCount};`}
              role="button"
              tabindex="0"
              on:click={() => openDay(day)}
              on:keydown={(event) => handleDayKeydown(event, day)}
            >
              <div class="day-head">
                <span class="day-number">{day.getDate()}</span>
                <span class="telework-labels">
                  {#if status.papa}<em>パパテレワーク</em>{/if}
                  {#if status.mama}<em>ママテレワーク</em>{/if}
                </span>
                {#if holidays[key]}
                  <span class="holiday-name">{holidays[key]}</span>
                {/if}
              </div>
              <div class="day-items">
                {#each scheduleForDate(key) as item}
                  <span class={colorClass(item.color)}>{item.text}</span>
                {/each}
              </div>
            </div>
          {/each}
          {#if rowSegments.length > 0}
            <div class="multi-overlay">
              {#each rowSegments as segment}
                <div
                  class={`multi-bar ${colorClass(segment.color)}`}
                  role="button"
                  tabindex="0"
                  style={`left: calc(${(segment.startCol / 7) * 100}% + 18px); width: calc(${((segment.endCol - segment.startCol + 1) / 7) * 100}% - 36px); top: ${44 + segment.lane * 24}px;`}
                  on:click={() => openMultiDay(segment.id)}
                  on:keydown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      openMultiDay(segment.id);
                    }
                  }}
                >
                  <span>{segment.text}</span>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </section>
</main>

<Modal title={`${selectedDayKey} の編集`} bind:open={dayModalOpen} size="lg">
  <div class="day-editor">
    <div class="telework-editor">
      <Checkbox checked={selectedTelework.papa} on:change={(e) => setTelework('papa', e.currentTarget.checked)}>パパテレワーク</Checkbox>
      <Checkbox checked={selectedTelework.mama} on:change={(e) => setTelework('mama', e.currentTarget.checked)}>ママテレワーク</Checkbox>
    </div>

    <div class="editor-switch" role="tablist" aria-label="予定種別">
      <button class:active={dayEditorMode === 'schedule'} type="button" on:click={() => switchDayEditorMode('schedule')}>日予定</button>
      <button class:active={dayEditorMode === 'multi'} type="button" on:click={() => switchDayEditorMode('multi')}>期間予定</button>
    </div>

    {#if dayModalMessage}
      <p class="modal-status">{dayModalMessage}</p>
    {/if}

    {#if dayEditorMode === 'schedule'}
      <div class="schedule-add">
        <Input
          class="schedule-text-input"
          style={`color: ${colorHex(newScheduleColor)};`}
          placeholder="予定を入力"
          bind:value={newScheduleText}
          maxlength="120"
        />
        <div class="color-palette" role="radiogroup" aria-label="追加する予定の色">
          {#each colorOptions as color}
            <button
              type="button"
              class={`color-swatch color-swatch-${color}`}
              class:active={newScheduleColor === color}
              role="radio"
              aria-checked={newScheduleColor === color}
              aria-label={colorLabels[color]}
              title={colorLabels[color]}
              on:click={() => (newScheduleColor = color)}
            >
              <span></span>
            </button>
          {/each}
        </div>
        <Button color="dark" on:click={addSchedule}>
          <Plus size={16} class="mr-2" />
          追加
        </Button>
      </div>

      <div class="schedule-list">
        {#each selectedItems as item (item.id)}
          <div class:dragging={draggingScheduleId === item.id} class:drag-over={dragOverScheduleId === item.id} class="schedule-row" data-schedule-id={item.id}>
            <button
              class="drag-handle"
              type="button"
              aria-label="予定の順番を変更"
              title="ドラッグして順番を変更"
              disabled={selectedItems.length < 2}
              on:pointerdown={(event) => startScheduleDrag(event, item.id)}
              on:keydown={(event) => handleScheduleHandleKeydown(event, item.id)}
            >
              <GripVertical size={18} />
            </button>
            <Input
              class="schedule-text-input"
              style={`color: ${colorHex(item.color)};`}
              value={item.text}
              maxlength="120"
              on:input={(e) => updateSchedule(item.id, { text: e.currentTarget.value })}
            />
            <div class="color-palette" role="radiogroup" aria-label="予定の色">
              {#each colorOptions as color}
                <button
                  type="button"
                  class={`color-swatch color-swatch-${color}`}
                  class:active={item.color === color}
                  role="radio"
                  aria-checked={item.color === color}
                  aria-label={colorLabels[color]}
                  title={colorLabels[color]}
                  on:click={() => updateSchedule(item.id, { color })}
                >
                  <span></span>
                </button>
              {/each}
            </div>
            <Button color="red" on:click={() => deleteSchedule(item.id)} aria-label="予定を削除">
              <Trash2 size={16} />
            </Button>
          </div>
        {/each}
        {#if selectedItems.length === 0}
          <p class="empty">この日の予定はまだありません。</p>
        {/if}
      </div>
    {:else}
      <div class="multi-editor-form">
        <div class="field">
          <span>開始日</span>
          <Input type="date" bind:value={newMultiDay.startDate} />
        </div>
        <div class="field">
          <span>終了日</span>
          <Input type="date" bind:value={newMultiDay.endDate} />
        </div>
        <div class="field">
          <span>予定名</span>
          <Input placeholder="予定名（空欄可）" bind:value={newMultiDay.text} maxlength="80" />
        </div>
        <div class="field">
          <span>色</span>
          <div class="color-palette" role="radiogroup" aria-label="期間予定の色">
            {#each colorOptions as color}
              <button
                type="button"
                class={`color-swatch color-swatch-${color}`}
                class:active={newMultiDay.color === color}
                role="radio"
                aria-checked={newMultiDay.color === color}
                aria-label={colorLabels[color]}
                title={colorLabels[color]}
                on:click={() => (newMultiDay.color = color)}
              >
                <span></span>
              </button>
            {/each}
          </div>
        </div>
        <Button color="dark" on:click={addMultiDay}>
          <Plus size={16} class="mr-2" />
          期間予定を追加
        </Button>
      </div>
    {/if}
  </div>
</Modal>

<Modal title="期間予定の編集" bind:open={multiDayModalOpen} size="md">
  {#if selectedMultiDayItem}
    <div class="multi-editor-form">
      <div class="field">
        <span>開始日</span>
        <Input type="date" value={selectedMultiDayItem.startDate} on:change={(e) => updateMultiDay(selectedMultiDayItem.id, { startDate: e.currentTarget.value })} />
      </div>
      <div class="field">
        <span>終了日</span>
        <Input type="date" value={selectedMultiDayItem.endDate} on:change={(e) => updateMultiDay(selectedMultiDayItem.id, { endDate: e.currentTarget.value })} />
      </div>
      <div class="field">
        <span>予定名</span>
        <Input value={selectedMultiDayItem.text} placeholder="予定名" maxlength="80" on:input={(e) => updateMultiDay(selectedMultiDayItem.id, { text: e.currentTarget.value })} />
      </div>
      <div class="field">
        <span>色</span>
        <div class="color-palette" role="radiogroup" aria-label="期間予定の色">
          {#each colorOptions as color}
            <button
              type="button"
              class={`color-swatch color-swatch-${color}`}
              class:active={selectedMultiDayItem.color === color}
              role="radio"
              aria-checked={selectedMultiDayItem.color === color}
              aria-label={colorLabels[color]}
              title={colorLabels[color]}
              on:click={() => updateMultiDay(selectedMultiDayItem.id, { color })}
            >
              <span></span>
            </button>
          {/each}
        </div>
      </div>
      <div class="modal-actions">
        <Button color="red" on:click={() => deleteMultiDay(selectedMultiDayItem.id)} aria-label="期間予定を削除">
          <Trash2 size={16} class="mr-2" />
          削除
        </Button>
      </div>
    </div>
  {/if}
</Modal>

{#if importBusy}
  <div class="blocking-overlay" role="alert" aria-live="assertive" aria-busy="true">
    <div class="blocking-dialog">
      <div class="spinner" aria-hidden="true"></div>
      <div>
        <strong>JSON を読み込んでいます</strong>
        <p>処理が完了するまでそのままお待ちください。</p>
      </div>
    </div>
  </div>
{/if}

<Modal title={importResultTitle} bind:open={importResultOpen} size="sm">
  <div class="result-dialog">
    <p>{importResultMessage}</p>
    <Button color="dark" on:click={() => (importResultOpen = false)}>閉じる</Button>
  </div>
</Modal>

<style>
  .app-shell {
    max-width: 1480px;
    margin: 0 auto;
    padding: 20px;
  }

  .toolbar,
  .panel {
    background: #ffffff;
    border: 1px solid #d1d5db;
    border-radius: 8px;
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 20px;
    padding: 16px;
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  h1,
  p {
    margin: 0;
  }

  h1 {
    font-size: 22px;
    font-weight: 800;
  }

  .title-row p {
    color: #4b5563;
    font-size: 13px;
  }

  .controls,
  .multi-form,
  .multi-row,
  .schedule-add,
  .schedule-row {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .controls > label {
    display: grid;
    gap: 4px;
    color: #374151;
    font-size: 12px;
    font-weight: 700;
  }

  .select-like {
    height: 42px;
    min-width: 104px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    color: #111827;
    padding: 0 10px;
    font-size: 14px;
  }

  .year-input {
    width: 112px;
  }

  .status {
    margin-top: 12px;
    padding: 10px 12px;
    border: 1px solid #fecaca;
    border-radius: 6px;
    background: #fef2f2;
    color: #991b1b;
    font-size: 13px;
  }

  .modal-status {
    padding: 10px 12px;
    border: 1px solid #fecaca;
    border-radius: 6px;
    background: #fef2f2;
    color: #991b1b;
    font-size: 13px;
    font-weight: 700;
  }

  .calendar-preview {
    margin-top: 16px;
    background: #fffdf7;
    border: 1px solid #c7c2b8;
    padding: 24px 18px 18px;
    box-shadow: 0 8px 20px rgb(17 24 39 / 8%);
  }

  .calendar-title {
    display: flex;
    justify-content: center;
    align-items: baseline;
    gap: 34px;
    height: 54px;
    font-weight: 800;
  }

  .calendar-title span {
    font-size: 28px;
  }

  .calendar-title strong {
    font-size: 52px;
    line-height: 1;
  }

  .weekday-grid,
  .week-row {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
  }

  .weekday-grid > div {
    border: 1px solid #1f2937;
    border-bottom: 0;
    border-left: 0;
    background: #e5e7eb;
    padding: 7px 4px;
    text-align: center;
    font-weight: 800;
  }

  .weekday-grid > div:first-child {
    border-left: 1px solid #1f2937;
  }

  .weekday-grid .weekday-sat {
    background: #476b9e;
    color: #ffffff;
  }

  .weekday-grid .weekday-sun {
    background: #a25a4b;
    color: #ffffff;
  }

  .week-row {
    position: relative;
    isolation: isolate;
    min-height: clamp(104px, 12vw, 136px);
  }

  .day-cell {
    display: block;
    position: relative;
    z-index: 1;
    min-width: 0;
    width: 100%;
    min-height: clamp(104px, 12vw, 136px);
    align-self: stretch;
    border: 1px solid #1f2937;
    border-top: 0;
    border-left: 0;
    background: #fffdf7;
    padding: 7px 7px 5px;
    text-align: left;
    vertical-align: top;
    overflow: hidden;
    cursor: pointer;
  }

  .day-cell:first-child {
    border-left: 1px solid #1f2937;
  }

  .day-cell.sat {
    background: #f7fbff;
  }

  .day-cell.sun-holiday {
    background: #fff8f8;
  }

  .day-cell.outside {
    background: #f3f4f6;
  }

  .day-head {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    min-height: 31px;
  }

  .day-number {
    color: #111827;
    font-size: 30px;
    font-weight: 900;
    line-height: 1;
  }

  .sat .day-number {
    color: #365f91;
  }

  .sun-holiday .day-number,
  .holiday-name {
    color: #9f2f2d;
  }

  .day-cell.outside .day-number,
  .day-cell.outside .holiday-name {
    color: #9ca3af;
  }

  .telework-labels {
    display: grid;
    gap: 1px;
    padding-top: 2px;
    color: #374151;
    font-size: 11px;
    font-style: normal;
    font-weight: 700;
    line-height: 1.2;
  }

  .telework-labels em {
    font-style: normal;
  }

  .holiday-name {
    min-width: 0;
    padding-top: 2px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: 12px;
    font-weight: 800;
    line-height: 1.2;
  }

  .day-items {
    display: grid;
    gap: 2px;
    margin-top: 8px;
    font-size: 13px;
    line-height: 1.25;
  }

  .day-cell.has-multi-day .day-items {
    margin-top: calc(8px + var(--day-multi-lanes) * 24px);
  }

  .day-items span {
    overflow-wrap: anywhere;
  }

  .item-black {
    color: #111111;
    border-color: #111111;
  }

  .item-red {
    color: #b91c1c;
    border-color: #b91c1c;
  }

  .item-blue {
    color: #1d4ed8;
    border-color: #1d4ed8;
  }

  .multi-overlay {
    position: absolute;
    inset: 0;
    z-index: 10;
    pointer-events: none;
  }

  .multi-bar {
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 20px;
    min-width: 28px;
    border: 1px solid currentColor;
    border-radius: 3px;
    background: #e5e7eb;
    cursor: pointer;
    pointer-events: auto;
  }

  .multi-bar.item-red {
    background: #fee2e2;
  }

  .multi-bar.item-blue {
    background: #dbeafe;
  }

  .multi-bar span {
    min-width: 0;
    max-width: calc(100% - 10px);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: #111827;
    font-size: 12px;
    font-weight: 700;
    line-height: 1;
  }

  .multi-bar:focus-visible {
    outline: 3px solid #2563eb;
    outline-offset: 2px;
  }

  .panel {
    padding: 16px;
  }

  .toolbar-data {
    position: relative;
    display: flex;
    align-items: center;
    gap: 10px;
    border-left: 1px solid #d1d5db;
    margin-left: 4px;
    padding-left: 14px;
  }

  .toolbar-data-menu {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    z-index: 40;
    display: grid;
    gap: 10px;
    min-width: 280px;
    border: 1px solid #d1d5db;
    border-radius: 8px;
    background: #ffffff;
    padding: 10px;
    box-shadow: 0 12px 28px rgb(17 24 39 / 18%);
  }

  .toolbar-data-menu :global(button),
  .toolbar-data-menu .file-button {
    width: 100%;
    justify-content: center;
  }

  .data-chevron {
    transition: transform 0.16s ease;
  }

  .data-chevron.open {
    transform: rotate(180deg);
  }

  .multi-form,
  .multi-row {
    align-items: center;
  }

  .multi-list {
    display: grid;
    gap: 8px;
    margin-top: 12px;
  }

  .multi-row {
    padding: 8px;
    border: 1px solid #e5e7eb;
    border-radius: 6px;
    background: #f9fafb;
  }

  .editor-switch {
    display: inline-flex;
    gap: 2px;
    width: fit-content;
    border: 1px solid #d1d5db;
    border-radius: 7px;
    background: #f3f4f6;
    padding: 2px;
  }

  .editor-switch button {
    min-width: 92px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    padding: 8px 12px;
    color: #374151;
    font-size: 14px;
    font-weight: 800;
    cursor: pointer;
  }

  .editor-switch button.active {
    background: #ffffff;
    color: #111827;
    box-shadow: 0 1px 2px rgb(17 24 39 / 12%);
  }

  .multi-editor-form {
    display: grid;
    gap: 12px;
  }

  .multi-editor-form .field {
    display: grid;
    gap: 5px;
    color: #374151;
    font-size: 12px;
    font-weight: 800;
  }

  .multi-editor-form .select-like {
    width: 100%;
  }

  .color-palette {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    width: fit-content;
    min-height: 42px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    padding: 5px;
  }

  .color-swatch {
    display: inline-grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border: 2px solid transparent;
    border-radius: 6px;
    background: transparent;
    padding: 0;
    cursor: pointer;
  }

  .color-swatch span {
    width: 18px;
    height: 18px;
    border: 1px solid rgb(17 24 39 / 20%);
    border-radius: 4px;
  }

  .color-swatch.active {
    border-color: #111827;
    background: #f3f4f6;
  }

  .color-swatch:focus-visible {
    outline: 3px solid rgb(37 99 235 / 35%);
    outline-offset: 2px;
  }

  .color-swatch-black span {
    background: #111111;
  }

  .color-swatch-red span {
    background: #b91c1c;
  }

  .color-swatch-blue span {
    background: #1d4ed8;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
  }

  .blocking-overlay {
    position: fixed;
    inset: 0;
    z-index: 9999;
    display: grid;
    place-items: center;
    background: rgb(17 24 39 / 48%);
    padding: 20px;
  }

  .blocking-dialog {
    display: flex;
    align-items: center;
    gap: 14px;
    width: min(420px, 100%);
    border: 1px solid #d1d5db;
    border-radius: 8px;
    background: #ffffff;
    padding: 18px;
    box-shadow: 0 18px 40px rgb(17 24 39 / 24%);
  }

  .blocking-dialog strong {
    display: block;
    color: #111827;
    font-size: 16px;
    font-weight: 800;
  }

  .blocking-dialog p,
  .result-dialog p {
    color: #4b5563;
    font-size: 14px;
  }

  .spinner {
    width: 28px;
    height: 28px;
    border: 3px solid #d1d5db;
    border-top-color: #1f2937;
    border-radius: 999px;
    animation: spin 0.8s linear infinite;
    flex: 0 0 auto;
  }

  .result-dialog {
    display: grid;
    gap: 16px;
  }

  .result-dialog :global(button) {
    justify-self: end;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .file-button {
    display: inline-flex;
    align-items: center;
    height: 42px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    padding: 0 14px;
    color: #111827;
    font-size: 14px;
    font-weight: 700;
    cursor: pointer;
    transition:
      background-color 0.15s ease,
      border-color 0.15s ease,
      box-shadow 0.15s ease;
  }

  .file-button:hover {
    border-color: #9ca3af;
    background: #f3f4f6;
  }

  .file-button:focus-within {
    border-color: #2563eb;
    box-shadow: 0 0 0 3px rgb(37 99 235 / 20%);
  }

  .file-button input {
    display: none;
  }

  .day-editor,
  .schedule-list {
    display: grid;
    gap: 14px;
  }

  .telework-editor {
    display: flex;
    gap: 18px;
    flex-wrap: wrap;
  }

  .schedule-add {
    align-items: center;
  }

  .schedule-row {
    display: grid;
    grid-template-columns: 34px minmax(0, 1fr) minmax(72px, 96px) auto;
    align-items: center;
    gap: 8px;
    padding: 8px;
    border: 1px solid #e5e7eb;
    border-radius: 6px;
    background: #f9fafb;
  }

  .schedule-row.dragging {
    border-color: #60a5fa;
    background: #eff6ff;
  }

  .schedule-row.drag-over {
    box-shadow: inset 0 2px 0 #2563eb;
  }

  .drag-handle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 42px;
    border: 1px solid #d1d5db;
    border-radius: 6px;
    background: #ffffff;
    color: #4b5563;
    cursor: grab;
    touch-action: none;
  }

  .drag-handle:active {
    cursor: grabbing;
  }

  .drag-handle:disabled {
    cursor: default;
    opacity: 0.35;
  }

  .schedule-row .color-palette {
    min-width: 0;
    width: 100%;
    justify-content: center;
  }

  .empty {
    color: #6b7280;
    font-size: 13px;
  }

  @media (max-width: 980px) {
    .toolbar {
      display: block;
    }

    .controls {
      margin-top: 14px;
    }

    .toolbar-data {
      width: 100%;
      border-left: 0;
      border-top: 1px solid #d1d5db;
      margin-left: 0;
      margin-top: 4px;
      padding-left: 0;
      padding-top: 12px;
    }

    .toolbar-data-menu {
      position: static;
      width: 100%;
      margin-top: 8px;
      box-shadow: none;
    }

    .calendar-preview {
      overflow-x: auto;
    }

    .weekday-grid,
    .week-stack {
      min-width: 920px;
    }
  }
</style>
