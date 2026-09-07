import {
  defaultLocalCalendarDateRange,
  localCalendarRangeFromDateInputs,
  type LocalCalendarDateRange,
  type LocalCalendarRequestRange,
} from './local-date-range'

export type MatchingDateRange = LocalCalendarDateRange
export type MatchingRequestRange = LocalCalendarRequestRange

export const defaultMatchingDateRange = defaultLocalCalendarDateRange
export const matchingRangeFromDateInputs = localCalendarRangeFromDateInputs
