import gettingStarted from './content/getting-started.md?raw'

export interface LightBridgeDoc {
  id: string
  title: string
  description?: string
  content: string
}

export const lightBridgeDocs: LightBridgeDoc[] = [
  {
    id: 'getting-started',
    title: '文档中心',
    description: 'LightBridge 模块与功能文档入口',
    content: gettingStarted,
  },
]

export function findDocById(id: string | null | undefined): LightBridgeDoc {
  return lightBridgeDocs.find((doc) => doc.id === id) ?? lightBridgeDocs[0]
}
