// +build js,wasm

package wasmdriver

import (
	"fmt"
	"unsafe"

	"github.com/nuberu/webgl"
	"github.com/nuberu/webgl/types"
)

const (
	textureUnitRGBA = webgl.TEXTURE0
	textureUnitY    = webgl.TEXTURE1
	textureUnitU    = webgl.TEXTURE2
	textureUnitV    = webgl.TEXTURE3
)

var elementsIndices = []uint16{
	0, 1, 3, // first triangle
	1, 2, 3, // second triangle
}

func (w *windowImpl) createBuffers() *types.VertexArray {
	vertexArray := w.gl.CreateVertexArray()
	vertexBuffer := w.gl.CreateBuffer()
	elementBuffer := w.gl.CreateBuffer()

	w.gl.BindVertexArray(vertexArray)

	w.gl.BindBuffer(webgl.ARRAY_BUFFER, vertexBuffer)
	w.gl.BufferData(webgl.ARRAY_BUFFER, texCoordsVertices, webgl.STATIC_DRAW)

	w.gl.BindBuffer(webgl.ELEMENT_ARRAY_BUFFER, elementBuffer)
	w.gl.BufferDataUI16(webgl.ELEMENT_ARRAY_BUFFER, elementsIndices, webgl.STATIC_DRAW)

	var emptyFloat float32
	floatLen := int(unsafe.Sizeof(emptyFloat))
	w.gl.VertexAttribPointer(aPositionIndex, 2, webgl.FLOAT, false, 4*floatLen, 0)
	w.gl.EnableVertexAttribArray(aPositionIndex)
	w.gl.VertexAttribPointer(aTexCoordsIndex, 2, webgl.FLOAT, false, 4*floatLen, 2*floatLen)
	w.gl.EnableVertexAttribArray(aTexCoordsIndex)

	return vertexArray
}

func (w *windowImpl) createAndAttachFragmentShader(shaderProgram *types.Program, fragmentShaderCode string) error {
	fragShader := w.gl.CreateFragmentShader()
	w.gl.ShaderSource(fragShader, fragmentShaderCode)
	w.gl.CompileShader(fragShader)
	if !w.gl.GetShaderParameterCompileStatus(fragShader) {
		return fmt.Errorf("Fragment shader: %s", w.gl.GetShaderInfoLog(fragShader))
	}
	w.gl.AttachShader(shaderProgram, fragShader)
	// this won't actually delete the shaders until the program is closed but it's a good practice
	w.gl.DeleteShader(fragShader)

	return nil
}

func (w *windowImpl) createTexture(
	unit types.GLEnum,
	format types.GLEnum,
	width int,
	height int,
) *types.Texture {
	w.gl.ActiveTexture(uint32(unit))
	imageTex := w.gl.CreateTexture()
	w.gl.BindTexture(webgl.TEXTURE_2D, imageTex)
	w.gl.TexParameterWrapS(webgl.TEXTURE_2D, webgl.REPEAT)
	w.gl.TexParameterWrapT(webgl.TEXTURE_2D, webgl.REPEAT)
	w.gl.TexParameterMinFilter(webgl.TEXTURE_2D, webgl.LINEAR)
	w.gl.TexParameterMagFilter(webgl.TEXTURE_2D, webgl.LINEAR)
	w.gl.TexImage2Db(webgl.TEXTURE_2D, 0, format, width, height, 0, format, nil)

	return imageTex
}
