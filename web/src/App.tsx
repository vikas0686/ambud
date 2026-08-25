import { NodeList } from './components/NodeList'
import { RegisterNodeForm } from './components/RegisterNodeForm'
import { DeployForm } from './components/DeployForm'
import { WorkloadList } from './components/WorkloadList'

function App() {
  return (
    <main className="dashboard">
      <h1>Ambud</h1>

      <section>
        <h2>Nodes</h2>
        <NodeList />
      </section>

      <section>
        <h2>Add a node</h2>
        <RegisterNodeForm />
      </section>

      <section>
        <h2>Deploy</h2>
        <DeployForm />
      </section>

      <section>
        <h2>Workloads</h2>
        <WorkloadList />
      </section>
    </main>
  )
}

export default App
