import { apiClient } from '../client'

export interface DailyAvailabilityPoint {
  date: string
  total_checks: number
  ok_count: number
  availability: number // 0..1
}

export interface RecentAvailabilityResponse {
  days: number
  points: DailyAvailabilityPoint[]
}

export async function getRecentAvailability(days: number = 30): Promise<RecentAvailabilityResponse> {
  const { data } = await apiClient.get<RecentAvailabilityResponse>('/admin/channel-monitors/availability', {
    params: { days }
  })
  return data
}

export const availabilityAPI = {
  getRecentAvailability,
}

export default availabilityAPI
