// +build js,wasm

package wasmdriver

import (
	"fmt"

	"github.com/nuberu/webgl/types"
)

const (
	aPositionIndex  = 0
	aTexCoordsIndex = 1
)

var texCoordsVertices = []float32{
	// positions // texture coords
	1.0, 1.0, 1.0, 0.0, // top right
	1.0, -1.0, 1.0, 1.0, // bottom right
	-1.0, -1.0, 0.0, 1.0, // bottom left
	-1.0, 1.0, 0.0, 0.0, // bottom left
}

const vertexShaderCode = `
attribute vec2 a_postion;
attribute vec2 a_texCoord;
varying vec2 v_texCoord;

void main() {
	gl_Position = vec4(a_postion, 0.0, 1.0);
	v_texCoord = a_texCoord;
}
`

func (w *windowImpl) createAndAttachVertexShader(shaderProgram *types.Program) error {
	vertShader := w.gl.CreateVertexShader()
	w.gl.ShaderSource(vertShader, vertexShaderCode)
	w.gl.CompileShader(vertShader)
	if !w.gl.GetShaderParameterCompileStatus(vertShader) {
		return fmt.Errorf("Vertex shader: %s", w.gl.GetShaderInfoLog(vertShader))
	}
	w.gl.AttachShader(shaderProgram, vertShader)
	w.gl.BindAttribLocation(shaderProgram, aPositionIndex, "a_position")
	w.gl.BindAttribLocation(shaderProgram, aTexCoordsIndex, "a_texCoord")

	// this won't actually delete the shaders until the program is closed but it's a good practice
	w.gl.DeleteShader(vertShader)

	return nil
}
