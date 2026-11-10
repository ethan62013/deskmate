import type { Object3D } from 'three'

export type PetKind = 'cat' | 'dog' | 'parrot'

export type PetState = 'idle' | 'walk' | 'happy' | 'sit' | 'sleep' | 'talk' | 'remind'

export interface PetModel {
  root: Object3D
  /** look is the pointer's horizontal position in the window, from -1 (left) to 1 (right). */
  update(dt: number, state: PetState, stateTime: number, look: number): void
  dispose(): void
}

export function disposeTree(root: Object3D) {
  root.traverse((o: any) => {
    o.geometry?.dispose?.()
    const mats = Array.isArray(o.material) ? o.material : o.material ? [o.material] : []
    for (const m of mats) {
      for (const v of Object.values(m)) (v as any)?.isTexture && (v as any).dispose()
      m.dispose()
    }
  })
}
