import * as THREE from 'three'
import { loadGltfPet } from './gltf'
import { createProceduralPet } from './procedural'
import type { PetKind, PetModel, PetState } from './types'

export interface ScreenRect {
  x: number
  y: number
  w: number
  h: number
}

function shadowTexture() {
  const c = document.createElement('canvas')
  c.width = c.height = 128
  const g = c.getContext('2d')!
  const grad = g.createRadialGradient(64, 64, 4, 64, 64, 64)
  grad.addColorStop(0, 'rgba(0,0,0,0.35)')
  grad.addColorStop(1, 'rgba(0,0,0,0)')
  g.fillStyle = grad
  g.fillRect(0, 0, 128, 128)
  return new THREE.CanvasTexture(c)
}

export class PetStage {
  private renderer: THREE.WebGLRenderer
  private scene = new THREE.Scene()
  private camera = new THREE.PerspectiveCamera(28, 1, 0.1, 50)
  private holder = new THREE.Group()
  private model: PetModel | null = null
  private clock = new THREE.Clock()
  private frame = 0
  private loadToken = 0
  private facing = 0
  private look = 0
  private box = new THREE.Box3()
  private shadow: THREE.Mesh

  state: PetState = 'idle'
  private stateTime = 0

  constructor(private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true })
    this.renderer.setClearColor(0x000000, 0)
    this.renderer.setPixelRatio(window.devicePixelRatio)
    this.renderer.outputColorSpace = THREE.SRGBColorSpace

    this.camera.position.set(0, 1.35, 5.5)
    this.camera.lookAt(0, 1.05, 0)

    this.scene.add(new THREE.HemisphereLight(0xffffff, 0xffe2c4, 1.6))
    const sun = new THREE.DirectionalLight(0xffffff, 2.2)
    sun.position.set(2, 4, 5)
    this.scene.add(sun)

    this.shadow = new THREE.Mesh(
      new THREE.PlaneGeometry(1.5, 1),
      new THREE.MeshBasicMaterial({ map: shadowTexture(), transparent: true, depthWrite: false }),
    )
    this.shadow.rotation.x = -Math.PI / 2
    this.shadow.position.y = 0.002
    this.scene.add(this.shadow, this.holder)

    this.resize()
    window.addEventListener('resize', this.resize)
    this.loop()
  }

  async setPet(kind: PetKind) {
    const token = ++this.loadToken
    const model = (await loadGltfPet(`/models/${kind}.glb`)) ?? createProceduralPet(kind)
    if (token !== this.loadToken) {
      model.dispose()
      return
    }
    if (this.model) {
      this.holder.remove(this.model.root)
      this.model.dispose()
    }
    this.model = model
    this.holder.add(model.root)
  }

  setState(state: PetState) {
    if (state === this.state && state !== 'happy') return
    this.state = state
    this.stateTime = 0
  }

  /** -1 faces left, 1 faces right, 0 faces the viewer. */
  face(dir: number) {
    this.facing = dir
  }

  setLook(clientX: number) {
    this.look = (clientX / this.canvas.clientWidth) * 2 - 1
  }

  /** The on-screen bounding box in CSS pixels, relative to the window; withShadow also covers the ground shadow. */
  screenRect(withShadow = false): ScreenRect | null {
    if (!this.model) return null
    this.box.setFromObject(this.model.root)
    if (this.box.isEmpty()) return null
    if (withShadow) this.box.expandByObject(this.shadow)
    const w = this.canvas.clientWidth
    const h = this.canvas.clientHeight
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
    const v = new THREE.Vector3()
    for (let i = 0; i < 8; i++) {
      v.set(i & 1 ? this.box.max.x : this.box.min.x, i & 2 ? this.box.max.y : this.box.min.y, i & 4 ? this.box.max.z : this.box.min.z)
      v.project(this.camera)
      const sx = ((v.x + 1) / 2) * w
      const sy = ((1 - v.y) / 2) * h
      minX = Math.min(minX, sx); maxX = Math.max(maxX, sx)
      minY = Math.min(minY, sy); maxY = Math.max(maxY, sy)
    }
    return { x: minX, y: minY, w: maxX - minX, h: maxY - minY }
  }

  hit(clientX: number, clientY: number) {
    const r = this.screenRect()
    if (!r) return false
    const pad = 6
    return clientX >= r.x - pad && clientX <= r.x + r.w + pad && clientY >= r.y - pad && clientY <= r.y + r.h + pad
  }

  dispose() {
    cancelAnimationFrame(this.frame)
    window.removeEventListener('resize', this.resize)
    this.model?.dispose()
    this.renderer.dispose()
  }

  private resize = () => {
    const w = this.canvas.clientWidth
    const h = this.canvas.clientHeight
    this.renderer.setSize(w, h, false)
    this.camera.aspect = w / h
    this.camera.updateProjectionMatrix()
  }

  private loop = () => {
    this.frame = requestAnimationFrame(this.loop)
    const dt = Math.min(this.clock.getDelta(), 0.1)
    this.stateTime += dt
    const targetYaw = this.facing * 1.1
    this.holder.rotation.y += (targetYaw - this.holder.rotation.y) * (1 - Math.exp(-dt * 8))
    this.model?.update(dt, this.state, this.stateTime, this.look)
    this.renderer.render(this.scene, this.camera)
  }
}
