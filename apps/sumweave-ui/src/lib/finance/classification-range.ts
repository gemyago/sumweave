import {
  defaultLocalCalendarDateRange,
  localCalendarRangeFromDateInputs,
  type LocalCalendarDateRange,
  type LocalCalendarRequestRange,
} from './local-date-range'

export type ClassificationDateRange = LocalCalendarDateRange
export type ClassificationRequestRange = LocalCalendarRequestRange

export const defaultClassificationDateRange = defaultLocalCalendarDateRange
export const classificationRangeFromDateInputs = localCalendarRangeFromDateInputs
